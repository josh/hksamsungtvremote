# hksamsungtvremote

Turn off and on a Samsung KS9 Series TV with HomeKit.

This implementation targets the Samsung KS9 2016 TV series but _may_ work with other models.

# Installation

[Download the latest `hksamsungtvremote` release from GitHub](https://github.com/josh/hksamsungtvremote/releases).

# Usage

You'll need to configure a static IP address for the TV and make note of it's MAC address.

```sh
hksamsungtvremote -ip 1.2.3.4 -mac ab:cd:ef:12:34:56 -pin 12345678
```
