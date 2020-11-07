mew scoring engine
==================

Multi-Endpoint light-Weight scoring engine (mew).

1. Speed and usability come first.
2. Simple is beautiful.

Usage
-----

0. Install `mongodb` and download `mew` release.
1. Write configuration in `./mew.conf`.
2. `./mew`.

Configuration
-------------

```toml
```

TODO:
    checks
        actually write code for them and dont just test tcp
    scoring
        make scope-able by time (ex. start counting at this time)
        make it apparent which red team has hacked
    flags:
        submit per team
        allow red users



Screenshot
-----------

![Main Status Page](screenshots/status.png)


Notes
-----------
Thanks to [scorestack](https://github.com/scorestack/scorestack/) for some the checks code.
