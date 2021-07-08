# Yggmail

It's email, but not as you know it.

## Introduction

Yggmail is a single-binary all-in-one mail transfer agent which sends and receives email natively over the [Yggdrasil Network](https://yggdrasil-network.github.io/).

* Yggmail runs just about anywhere you like — your inbox is stored right on your own machine;
* Implements IMAP and SMTP protocols for sending and receiving mail, so you can use your favourite client (hopefully);
* Mails are exchanged between Yggmail users using built-in Yggdrasil connectivity;
* All mail exchange traffic between any two Yggmail nodes is always end-to-end encrypted without exception;
* Yggdrasil and Yggmail nodes on the same network are discovered automatically using multicast or you can configure a static Yggdrasil peer.

Email addresses are based on your public key, like `neilalexander@1mLp6AtYSE7rYOVDDTPKzasmFgG9BfKOk7aK4xOdZcT.yggmail`. 

## Why?

There are all sorts of messaging services in the world but there is still a lot of value in asynchronous communication. Email is something that a lot of people understand reasonably well and there is still a huge volume of software in the world which supports email. Yggmail is designed to comply with the standards that people know and expect.

Yggdrasil is well-suited for ad-hoc mail delivery and allows Yggmail to work even in closed networks, where Internet or other connectivity is restricted or simply not available. It guarantees end-to-end encryption and handles networks with changing topologies reasonably well. 

## Quickstart

Use a recent version of Go to install Yggmail:

```
go install github.com/neilalexander/yggmail/cmd/yggmail
```

Create a mailbox, e.g. for user `alice`. Your Yggmail database will automatically be created in your working directory if it doesn't already exist:
```
yggmail -createuser=alice
```

Start Yggmail, using the database in your working directory:
```
yggmail -smtp=localhost:1025 -imap=localhost:1026
```

Connect your mail client to Yggmail. In the above example:

* SMTP is listening on `localhost` port 1025, password authentication, no SSL/TLS
* IMAP is listening on `localhost` port 1026, password authentication, no SSL/TLS

Then try sending a mail to another Yggmail user!

## Parameters

The following command line switches are supported by the `yggmail` binary:

* `-peer=tls://...` or `-peer=tcp://...` — connect to a specific Yggdrasil node, like one of the [Public Peers](https://publicpeers.neilalexander.dev/);
* `-database=/path/to/yggmail.db` — use a specific database file;
* `-smtp=listenaddr:port` — listen for SMTP on a specific address/port
* `-imap=listenaddr:port` — listen for IMAP on a specific address/port;
* `-createuser=username` — create a new user in the database (doesn't matter if Yggmail is running or not, just make sure that Yggmail is pointing at the right database file or that you are in the right working directory).

## Notes

There are a few important notes:

* Yggmail needs to be running in order to receive inbound emails — it's therefore important to run Yggmail somewhere that will have good uptime;
* Yggmail tries to guarantee that senders are who they say they are. Your `From` address must be your Yggmail address (or at the very least, from your Yggmail domain);
* You can only email other Yggmail users, not regular email addresses on the public Internet;
* You may need to configure your client to allow "insecure" or "plaintext" authentication to IMAP/SMTP — this is because we don't support SSL/TLS on the IMAP/SMTP listeners yet;
* Yggmail won't transport mails larger than 1MB right now.

## Bugs

There are probably all sorts of bugs, but the ones that we know of are:

* IMAP behaviour might not be entirely spec-compliant in all cases, so your mileage with mail clients might vary;
* SMTP queues up outbound mails in memory rather than in the database right now — if you restart Yggmail, any unsent mails will be lost.

The code's also a bit of a mess, so sorry about that.
