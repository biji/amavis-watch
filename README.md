# About

Watch postfix log files containing amavis scanning result

# Install

Compile 

```bash
go mod download
make
```

Rsync `amavis-watch` `htpasswd.txt` `assets` `templates` to server

```bash
rsync -avP ./amavis-watch htpasswd.txt templates assets user@your-mail-server.com:
```

# Run

```bash
./amavis-watch /var/log/mail.log.1 /var/log/mail.log
```

Open using browser: `http://your-mail-server.com/index`

Run in background:

```bash
nohup ./amavis-watch /var/log/mail.log.1 /var/log/mail.log &
```

Optional parameters:

```
./amavis-watch [-cred FILENAME] [-prod] file.log.1 file.log

-cred FILENAME      specify htpasswd credential file
-prod               Run in production mode
```

# Change password

Default username is `amavis` with password `watch`

```bash
# change password
htpasswd htpasswd.txt amavis

# create new user
htpasswd htpasswd.txt username
```
