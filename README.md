# Chesshook intermediary

A simple server to run a chess engine and communicate with the chesshook userscript. Generates a random passkey on startup. The passkey is used to authenticate the userscript, and is printed to the console on startup. The server has hardcoded values whether the key is required for read or write access. By default the key is required for write access, but not for read access.

## Usage

### Version Checking

```
message: whoareyou 

reponse: iam <name>v<version>

Useful for checking if the server is outdated compared to the userscript.
``` 
### Authentication
```
message: `auth <passkey>`

response: `authok` or `autherr`

If the user fails to authenticate three times, every attempt will continue to fail.
```
### Subscribing to engine output
```
message: `sub`
response: `subok` or `suberr` or `autherr`

message: `unsub`
response: `unsubok` or `unsuberr` or `autherr`
```
### Locking the engine for others
```
message: `lock`
response: `lockok` or `lockerr` or `autherr`

message: `unlock`
response: `unlockok` or `unlockerr` or `autherr`
```
### UCI Commands
```
message: `<uci command>`

response: `autherr` or `<uci response>`
```