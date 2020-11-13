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
event = "DCDC Round 1" # event title
kind = "blue" # competition type (default "blue"))
                # can be one of "blue", "purple", "red"
                # for non-blue, red persists rob blue team of points, all red for a box means red gets double points

verbose = true # show more info to competitors
tightlipped = false # hide most informational output

delay = 20   # delay (seconds) between checks (>0) (default 60)
             # note: the "real" max delay will be timeout+delay+jitter
jitter = 3  # jitter (seconds) between rounds (0<jitter<delay)
timeout = 5 # check timeout
             # must be smaller than delay-jitter

slathreshold = 6 # how many checks before incurring SLA violation
slapoints = 12 # how many points is an SLA penalty (default sla_threshold * 2)

[[admin]]
name = "admin"
pw = "HACKTHEPLANET"

[[team]]
display = "Team1"
prefix = "10.20.1."
red = "40eb"
color = "#0af"
pw = "Team1Pw!"

[[team]]
display = "Team2"
prefix = "10.20.2."
pw = "AppleSauce"

[[creds]]
name = "users"
usernames = ["john", "hiss", "richard", "sheriff", "captain", "guards", "otto", "rabbits", "skippy", "tagalong", "kluck", "toby"]
defaultpw = "Password1!"

[[creds]]
name = "admins"
usernames = ["robin", "dale", "tuck"]
defaultpw = "Password1!"

[[box]]
name = "village"
suffix = "3"

    [[box.web]]
        [[box.web.url]]
        path="/joomla"
        regex="It's easy to get started creating your website. Knowing some of the basics will help."

    [[box.smb]]
    domain = "WORKGROUP"

[[box]]
name="castle"
suffix = "4"

    [[box.dns]]
        [[box.dns.record]]
        kind = "A"
        domain = "townsquare.sherwood.lan"
        answer = ["192.168.1.4",]

        [[box.dns.record]]
        kind = "A"
        domain = "castle.sherwood.lan"
        answer = ["192.168.1.5",]

    [[box.smb]]
    domain = "SHERWOOD.lan"


[[box]]
name="bridge"
suffix = "5"

    [[box.web]]
        [[box.web.url]]
        regex="Welcome to the super cool xkcd downloader. Here you can download XKCD comics"

    [[box.smb]]

    [[box.ssh]]
```


Screenshot
-----------

![Main Status Page](screenshots/status.png)


Notes
-----------
Thanks to [scorestack](https://github.com/scorestack/scorestack/) for some of the code for checks.
