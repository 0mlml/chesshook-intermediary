package main

import (
	"bufio"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	namespace = "chesshook-intermediary"
	version   = "1"

	addr     = flag.String("addr", "localhost:8080", "http service address")
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return r.Header.Get("Origin") == "https://www.chess.com"
		},
	}
	passKey           = randomPassKey()
	needsAuthForWrite = flag.Bool("authwrite", true, "whether to require authentication for writing to the engine")
	needsAuthForRead  = flag.Bool("authread", false, "whether to require authentication for reading from the engine")
	localhostBypass   = flag.Bool("localhost", true, "whether to bypass authentication for localhost")

	enginePath          = flag.String("engine", "./stockfish", "path to the engine binary")
	uciArgsFlag         = flag.String("uciargs", "", "arguments to pass to the engine on startup, split with \";\"")
	engineInputChannel  = make(chan string)
	engineOutputChannel = make(chan string)
	engineName          = ""
)

func randomPassKey() string {
	length := 10
	chars := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	passKey := make([]rune, length)
	for i := range passKey {
		passKey[i] = chars[rand.Intn(len(chars))]
	}
	return string(passKey)
}

func spawnEngine() {
	engineArgs := strings.Split(*enginePath, " ")
	engine := exec.Command(engineArgs[0], engineArgs[1:]...)
	stdin, err := engine.StdinPipe()

	if err != nil {
		panic(err)
	}

	stdout, err := engine.StdoutPipe()
	if err != nil {
		panic(err)
	}

	err = engine.Start()
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			uciInput := <-engineInputChannel

			_, err = fmt.Fprintf(stdin, "%s\n", uciInput)
			if err != nil {
				panic(err)
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			// Send the output to a channel for further processing
			output := scanner.Text()
			fmt.Println("engine: " + output)
			if strings.HasPrefix(output, "id name ") {
				engineName = output[8:]
			}
			if strings.HasPrefix(output, "bestmove") || strings.HasPrefix(output, "info") {
				engineOutputChannel <- output
			}
		}
	}()

	engineInputChannel <- "uci"
	engineInputChannel <- "isready"

	uciArgs := strings.Split(*uciArgsFlag, ";")
	for _, arg := range uciArgs {
		engineInputChannel <- arg
	}
}

var isEngineLocked = false

type User struct {
	connection *websocket.Conn
	mu         sync.Mutex

	isSubscribed bool
	hasLock      bool

	isAuthenticated   bool
	incorrectAttempts int
}

var users = make(map[*websocket.Conn]*User)

func writePump() {
	for {
		message := <-engineOutputChannel
		for _, user := range users {
			if user.isSubscribed {
				user.send(message)
			}
		}
	}
}

func (user *User) send(message string) bool {
	user.mu.Lock()
	defer user.mu.Unlock()

	err := user.connection.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Println("write:", err)
		return false
	}
	return true
}

func wsHandler(writer http.ResponseWriter, request *http.Request) {
	connection, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	connection.SetReadLimit(64 * 1024) // 64 KB max message size

	log.Print("New ws opened: ", connection.RemoteAddr())

	user := User{
		connection:        connection,
		isSubscribed:      false,
		hasLock:           false,
		isAuthenticated:   false,
		incorrectAttempts: 0,
	}

	if *localhostBypass && strings.HasPrefix(connection.RemoteAddr().String(), "127.0.0.1:") {
		user.isAuthenticated = true
	}

	users[connection] = &user

	defer func() {
		log.Print("ws closed: ", connection.RemoteAddr())
		if user.hasLock {
			isEngineLocked = false
		}
		connection.Close()
		delete(users, connection)
	}()

	for {
		_, message, err := connection.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		log.Printf("recv: %s", message)
		parts := strings.Split(string(message), " ")

		if parts[0] == "whoareyou" {
			if !user.send(fmt.Sprintf("iam %sv%s", namespace, version)) {
				break
			}
		} else if parts[0] == "whatengine" {
			if !user.isAuthenticated && *needsAuthForRead {
				if !user.send("autherr") {
					break
				}
				continue
			}
			if !user.send(fmt.Sprintf("engine %s", engineName)) {
				break
			}
		} else if parts[0] == "auth" {
			// Check if the passkey is correct, and if the user has not exceeded the number of incorrect attempts
			if parts[1] == passKey && user.incorrectAttempts < 3 {
				if !user.send("authok") {
					break
				}
				user.isAuthenticated = true
			} else {
				if !user.send("autherr") {
					break
				}
				user.incorrectAttempts++
			}
		} else if parts[0] == "sub" {
			if !user.isAuthenticated && *needsAuthForRead {
				if !user.send("autherr") {
					break
				}
				continue
			}

			if !user.isSubscribed {
				user.isSubscribed = true
				if !user.send("subok") {
					break
				}
			} else {
				if !user.send("suberr") {
					break
				}
			}
		} else if parts[0] == "unsub" {
			if !user.isAuthenticated && *needsAuthForRead {
				if !user.send("autherr") {
					break
				}
				continue
			}

			if user.isSubscribed {
				user.isSubscribed = false
				if !user.send("unsubok") {
					break
				}
			} else {
				if !user.send("unsuberr") {
					break
				}
			}
		} else if parts[0] == "lock" {
			if !user.isAuthenticated && *needsAuthForWrite {
				if !user.send("autherr") {
					break
				}
				continue
			}
			if !user.hasLock && !isEngineLocked {
				user.hasLock = true
				isEngineLocked = true
				if !user.send("lockok") {
					break
				}
			} else {
				if !user.send("lockerr") {
					break
				}
			}
		} else if parts[0] == "unlock" {
			if !user.isAuthenticated && *needsAuthForWrite {
				if !user.send("autherr") {
					break
				}
				continue
			}
			if user.hasLock {
				user.hasLock = false
				isEngineLocked = false
				if !user.send("unlockok") {
					break
				}
			} else {
				if !user.send("unlockerr") {
					break
				}
			}
		} else {
			if !user.isAuthenticated && *needsAuthForWrite {
				if !user.send("autherr") {
					break
				}
				continue
			}
			engineInputChannel <- string(message)
		}
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, *addr)
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/", home)
	go spawnEngine()
	go writePump()
	log.Print("Server started, passkey: ", passKey)
	log.Print("Server is requesting authentication for read operations: ", *needsAuthForRead)
	log.Print("Server is requesting authentication for write operations: ", *needsAuthForWrite)
	log.Print("Server is bypassing authentication for localhost connections: ", *localhostBypass)
	panic(http.ListenAndServe(*addr, nil))
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Chesshook External Engine server</title>
</head>
<body>
<p>This server is running a websocket server for the Chesshook userscript external engine protocol. You can find the chesshook userscript <a href="https://github.com/0mlml/chesshook">here.</a> You can find the source for this server <a href="https://github.com/0mlml/chesshook-intermediary">here.</a></p>
<p>To use this server with the userscript, set the engine url to <code>ws://{{.}}/ws</code></p>
</body>
</html>
`))
