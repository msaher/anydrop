# anydrop

Share files over your LAN.

Unlike solutions from multibillion-dollar corporations, this one doesn't nag you you about logins, restrict you to certain hardware, or push the cloud on you.

Motivation: Wanted to move a large file from my phone. Existing solutions were broken or forced me to use the internet and involve a third party for something so simple.

Want to let others download a file?

```
$ anydrop -download myfile
```

Want to receive file in a specific directory?

```
$ anydrop -upload-dir ~/docs
```

See `anydrop -help` for full usage

# Features

- Convenient CLI interface
- Pretty web UI
- QR code generation
- Token-based authentication
- Prevents filename collisions
- Single binary

# Installation

Build with

```
$ make
```

Install by running

```
$ make install
```

Set `PREFIX` to set an install destination. Example:

```
$ PREFIX=~/.local/ make install
```
