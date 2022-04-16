# l4la
[![GitHub release](https://img.shields.io/github/v/tag/hzyitc/l4la?label=release)](https://github.com/hzyitc/l4la/releases)

[README](README.md) | [中文文档](README_zh.md)

**!!! Note: this is a `just-work` version, all things may be changed without any backward-compatibility !!!**

## Introduction

`l4la` is a tool that divide a connection into multi connections.

It is useful for the people who has multi-wan at home and a high-bandwidth server in other place.

```
---------------    <----    ~~~~~~~~~~~~~~~~~~~~~    <----    ---------------
| l4la Server |    <----    { Multi connections }    <----    | l4la Client |
---------------    <----    ~~~~~~~~~~~~~~~~~~~~~    <----    ---------------
      |                                                              ^
      |                              V                               |
      |                                                              |
      |                              S                               |
      ↓                                                              |
  -----------                ~~~~~~~~~~~~~~~~~~                  ----------
  | Service |    <--------   { One connectiom }    <---------    | Client |
  -----------                ~~~~~~~~~~~~~~~~~~                  ----------
```

## Usage

```
Usage:
  l4la server <listen port> <service address>
  l4la client <listen port> <server address> <connection number>
```
