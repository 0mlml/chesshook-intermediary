# Chesshook intermediary

A simple server to run a chess engine and communicate with the chesshook userscript. Generates a random passkey on startup. The passkey is used to authenticate the userscript, and is printed to the console on startup. The server has hardcoded values whether the key is required for read or write access. By default the key is required for write access, but not for read access.

## Usage
- I have not included precompiled binaries. Please use [go](https://go.dev/dl/) to compile the program using `go build main.go`. 
- Command line flags:
    - `-help`: prints usage.
    - `-addr <string>`: the http service address. default: localhost:8080
    - `-engine <string>`:  path to the engine binary. you must include `./` for relative paths. default: stockfish
    - `-authwrite <bool>`: whether the passkey is required for write access. default: true
    - `-authread <bool>`: whether the passkey is required for read access. default: false
    - `-localhostbypass <bool>`: whether the passkey is required for localhost connections. default: true
    - `-uciargs <string>`: arguments to pass to the engine on startup, split with semicolons ";". Should look like "setoption name Skill Level value 20;setoption name Threads value 10"
- Place the engine executable in the same folder as the server executable.
- `./main -engine ./<name of engine>`
    - the output should be similar to:
    ```
    Server started, passkey: 4QZB13qLMj
    Server is requesting authentication for read operations: false
    Server is requesting authentication for write operations: true
    Server is bypassing authentication for localhost connections: true
    engine: Stockfish 15.1 by the Stockfish developers (see AUTHORS file)
    engine: id name Stockfish 15.1
    ...
    ```
- Configure the Chesshook userscript.
    - set the engine to external.
    - set the passkey to the passkey printed to your console. it will change every time the server is restarted.
        - this is not necessary if you do not set `-localhostbypass` to false, and the server is on localhost.
    - in the server is running on the same machine, the url of the engine is `ws://localhost:8080/ws`.
        - if the server is running on a different machine, replace `localhost` with the address of the machine.
    - go to the "external" page from the hamburger menu.
        - the top panel will report if you are connected to the server.
        - you should also see some messages like `New ws opened: 127.0.0.1:12345`, `recv: whoareyou`, and `recv: whatengine` in the console.
        - by default, you will only try to authenticate once your client recieves `autherr` from the server.
        - the client will not try to reconnect to server. you will need to refresh the page or change the engine option to reconnect.

## Developer Usage
More advanced users may find it helpful

### Version Checking

```
message: whoareyou 

reponse: iam <name>v<version>

message: whatengine

response: engine <name>

Useful for checking if the server is outdated compared to the userscript.
``` 
### Authentication
```
message: `auth <passkey>`
response: `authok` or `autherr`
If the server responds with `authok`, the connection has been authenticated.
If the server responds with `autherr`, the connection has not been authenticated. The passkey could be incorrect or the server could be stonewalling the connection.

If the user fails to authenticate three times, every attempt will continue to fail.
```
### Subscribing to engine output
```
message: `sub`
response: `subok` or `suberr` or `autherr`
If the server responds with `subok`, the client will recieve all engine output beginning with `bestmove` and `info` through the websocket.
If the server responds with `suberr`, the client is already subscribed
If the server responds with `autherr`, the client is expected to provide a passkey, as it is required for read access.  

message: `unsub`
response: `unsubok` or `unsuberr` or `autherr`
If the server responds with `unsubok`, the client will no longer recieve engine output.
If the server responds with `unsuberr`, the client is not subscribed
If the server responds with `autherr`, the client is expected to provide a passkey, as it is required for read access.  
```
### Locking the engine for others
```
message: `lock`
response: `lockok` or `lockerr` or `autherr`
If the server responds with `lockok`, the server will reject all uci commands from other connections.
If the server responds with `lockerr`, someone already has a lock on the server. the client is expected to maintain whether it is the one with the lock.
If the server responds with `autherr`, the client is expected to provide a passkey, as it is required for write access.  

message: `unlock`
response: `unlockok` or `unlockerr` or `autherr`
If the server responds with `unlockok`, the server will allow other connections to lock the server or send uci commands.
If the server responds with `unlockerr`, the client does not have lock on the server.
If the server responds with `autherr`, the client is expected to provide a passkey, as it is required for write access.  
```
### UCI Commands
```
message: `<uci command>`
response: `autherr`
If the server responds with `autherr`, the client is expected to provide a passkey, as it is required for write access.  

The client is expected to subscribe to engine output if it would like to recieve engine output.
```
