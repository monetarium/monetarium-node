monetarium
====

[![Build Status](https://github.com/monetarium/monetarium-node/workflows/Build%20and%20Test/badge.svg)](https://github.com/monetarium/monetarium-node/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![Doc](https://img.shields.io/badge/doc-reference-blue.svg)](https://pkg.go.dev/github.com/monetarium/monetarium-node)

## Monetarium Overview

Monetarium is a blockchain-based cryptocurrency implementing a dual-coin system
(VAR + SKA). It utilizes a hybrid proof-of-work and proof-of-stake mining system
to ensure that a small group cannot dominate the flow of transactions or make
changes to Monetarium without the input of the community. The primary unit of
the currency is called a `varta` (VAR).

## What is monetarium?

monetarium is a full node implementation of Monetarium written in Go (golang).

It acts as a fully-validating chain daemon for the Monetarium cryptocurrency.
monetarium maintains the entire past transactional ledger of Monetarium and
allows relaying of transactions to other Monetarium nodes around the world.

This software is currently under active development.

It is important to note that monetarium does *NOT* include wallet functionality.

## What is a full node?

The term 'full node' is short for 'fully-validating node' and refers to software
that fully validates all transactions and blocks, as opposed to trusting a 3rd
party.  In addition to validating transactions and blocks, nearly all full nodes
also participate in relaying transactions and blocks to other full nodes around
the world, thus forming the peer-to-peer network that is the backbone of the
Monetarium cryptocurrency.

The full node distinction is important, since full nodes are not the only type
of software participating in the Monetarium peer network. For instance, there are
'lightweight nodes' which rely on full nodes to serve the transactions, blocks,
and cryptographic proofs they require to function, as well as relay their
transactions to the rest of the global network.

## Why run monetarium?

As described in the previous section, the Monetarium cryptocurrency relies on having
a peer-to-peer network of nodes that fully validate all transactions and blocks
and then relay them to other full nodes.

Running a full node with monetarium contributes to the overall security of the
network, increases the available paths for transactions and blocks to relay,
and helps ensure there are an adequate number of nodes available to serve
lightweight clients, such as Simplified Payment Verification (SPV) wallets.

Without enough full nodes, the network could be unable to expediently serve
users of lightweight clients which could force them to have to rely on
centralized services that significantly reduce privacy and are vulnerable to
censorship.

In terms of individual benefits, since monetarium fully validates every block and
transaction, it provides the highest security and privacy possible when used in
conjunction with a wallet that also supports directly connecting to it in full
validation mode.

## Minimum Recommended Specifications (monetarium only)

* 16 GB disk space (increases over time)
* 2 GB memory (RAM)
* ~150 MB/day download, ~1.5 GB/day upload
  * Plus one-time initial download of the entire block chain
* Windows 10 (server preferred), macOS, Linux
* High uptime

## Getting Started

So, you've decided to help the network by running a full node.  Great!  Running
monetarium is simple.  All you need to do is install monetarium on a machine that is
connected to the internet and meets the minimum recommended specifications, and
launch it.

Also, make sure your firewall is configured to allow inbound connections to port
9108.

<a name="Installation" />

## Installing and updating

### Build from source (all platforms)

<details><summary><b>Install Dependencies</b></summary>

- **Go 1.23 or 1.24**

  Installation instructions can be found here: https://golang.org/doc/install.
  Ensure Go was installed properly and is a supported version:
  ```sh
  $ go version
  $ go env GOROOT GOPATH
  ```
  NOTE: `GOROOT` and `GOPATH` must not be on the same path. Since Go 1.8 (2016),
  `GOROOT` and `GOPATH` are set automatically, and you do not need to change
  them. However, you still need to add `$GOPATH/bin` to your `PATH` in order to
  run binaries installed by `go get` and `go install` (On Windows, this happens
  automatically).

  Unix example -- add these lines to .profile:

  ```
  PATH="$PATH:/usr/local/go/bin"  # main Go binaries ($GOROOT/bin)
  PATH="$PATH:$HOME/go/bin"       # installed Go projects ($GOPATH/bin)
  ```

- **Git**

  Installation instructions can be found at https://git-scm.com or
  https://gitforwindows.org.
  ```sh
  $ git version
  ```
</details>
<details><summary><b>Windows Example</b></summary>

  ```PowerShell
  PS> git clone https://github.com/monetarium/monetarium-node $env:USERPROFILE\src\monetarium
  PS> cd $env:USERPROFILE\src\monetarium
  PS> go install . .\cmd\...
  PS> monetarium -V
  ```

  Run the `monetarium` executable now installed in `"$(go env GOPATH)\bin"`.
</details>
<details><summary><b>Unix Example</b></summary>

  This assumes you have already added `$GOPATH/bin` to your `$PATH` as described
  in dependencies.

  ```sh
  $ git clone https://github.com/monetarium/monetarium-node $HOME/src/monetarium
  $ (cd $HOME/src/monetarium && go install . ./...)
  $ monetarium -V
  ```

  Run the `monetarium` executable now installed in `$GOPATH/bin`.
</details>

## Building and Running OCI Containers (aka Docker/Podman)

The project does not officially provide container images.  However, all of the
necessary files to build your own lightweight non-root container image based on
`scratch` from the latest source code are available in
[contrib/docker](./contrib/docker/README.md).

It is also worth noting that, to date, most users typically prefer to run `monetarium`
directly, without using a container, for at least a few reasons:

- `monetarium` is a static binary that does not require root privileges and therefore
  does not suffer from the usual deployment issues that typically make
  containers attractive
- It is harder and more verbose to run `monetarium` from a container as compared to
  normal:
  - `monetarium` is designed to automatically create a working default configuration
    which means it just works out of the box without the need for additional
    configuration for almost all typical users
  - The blockchain data and configuration files need to be persistent which
    means configuring and managing a docker data volume
  - Running non-root containers with `docker` requires special care in regards
    to permissions

## Running Tests

All tests and linters may be run using the script `run_tests.sh`.

```
./run_tests.sh
```

## Issue Tracker

The [integrated github issue tracker](https://github.com/monetarium/monetarium-node/issues)
is used for this project.
