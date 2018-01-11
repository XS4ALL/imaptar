
# imaptar

Imaptar is a utility to dump an entire IMAP mailbox, INBOX
and all folders, in maildir format to a tar file.

## Usage:

```
  Usage: imaptar <flags>
  
  Flags:\n\n" +
    -server <name>   IMAPS server name
    -port <port>     IMAPS server port (default 993)
    -user <name>     username
    -pass <pass>     password
    -tar <file>      tar output filename
```

## Example

```
imaptar -server imap.xs4all.nl -user mikevs -pass trustno1 -tar output.tar
```

## BUGS

Only works on IMAP servers where "/" is the folder seperator.

## Building

```
$ go get
$ go build
```

## Building a debian package

```
$ dpkg-buildpackage -rfakeroot -us -uc
```

