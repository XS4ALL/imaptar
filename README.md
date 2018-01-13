
# imaptar

Imaptar is a utility to dump an entire IMAP mailbox, INBOX
and all folders, in maildir format to a tar file.

## Usage:

```
Usage: imaptar <flags>

Flags:
    -s, --server <name>   IMAPS server name
    -u, --user <name>     username
    -t, --tar <file>      tar output filename

Optional flags:
    -p, --port <port>     IMAPS server port (default 993)
    -P, --pass <pass>     password
    -E, --envpass VAR     get password from environment var $VAR
    -z, --gzip            compress the output
```

## Example

```
imaptar -server imap.xs4all.nl -user mikevs -pass trustno1 -tar output.tar
```

## BUGS

Only works on IMAP servers where "/" is the folder seperator.

## Building

You need to have the 'gó' compiler installed, ofcourse. Then:

```
$ go get
$ go build
```

## Building a debian package

If you are running debian or ubuntu and you would like to have
a .deb format package, run:

```
$ dpkg-buildpackage -rfakeroot -us -uc
```

