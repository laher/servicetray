# servicetray

Configurable system tray widget for a group of related services

![screenshot](./servicetray.png)

## Installation

servicetray is a go app. It's built with [https://github.com/getlantern/systray](systray) - see that repo for installation requirements.

In due course I'll add `goreleaser` or similar, to make it easy to install this.

Please send a documentation PR if you manage to install servicetray on another platform.

### Ubuntu

I use Ubuntu Linux, so I did the following (from the above repo)

```bash
sudo apt-get install gcc libgtk-3-dev libappindicator3-dev
```

## Usage

Create a config file called `servicetray.yml`, before running `servicetray`

### Simple example

```yaml
title: VPNs
items:
   - name: wireguard-am
     start:
       cmd: systemctl
       args: [start,wg-quick@am]
     stop:
       cmd: systemctl
       args: [stop,wg-quick@am]
     check:
       cmd: systemctl
       args: [status,wg-quick@am]
```

### Other examples

Typically you'd group multiple similar items together.

This example uses a 'template' for managing multiple vpns, via systemctl ...

```yaml
title: VPNs
icon: /usr/share/icons/gnome/16x16/emblems/emblem-system.png
items:
   - name: wireguard-am
     template: systemctl
     vars:
       svc: wg-quick@am
   - name: openvpn-amir
     template: systemctl
     vars:
       svc: openvpn@am
   - name: openvpn-other
     template: systemctl
     vars:
       svc: openvpn@other
templates:
   - name: systemctl
     start:
       cmd: systemctl
       args: [start,"{{.svc}}"]
     stop:
       cmd: systemctl
       args: [stop,"{{.svc}}"]
     check:
       cmd: systemctl
       args: [status,"{{.svc}}"]
```


### Generators

This example uses docker-compose to dynamically generate a list of services at startup

```yaml
title: dev-project
icon: /usr/share/icons/docker/16x16/docker.png
pwd: /home/am/my-dockercompose-project
generators:
  - name: docker-compose
    find:
      cmd: docker-compose
      args: [ps,--services]
    template: docker-compose
templates:
  - name: docker-compose
    start:
      cmd: docker-compose
      args: [up,-d,"{{.svc}}"]
    stop:
      cmd: docker-compose
      args: [stop,"{{.svc}}"]
    check:
      cmd: /home/am/my-dockercompose-project/dcrunning.sh
      args: ["{{.svc}}"]
```

Note: docker-compose isn't the easiest for checking status, so this recipe uses a shell script. I included a bash script to give an idea of what to do ... [./helpers/dcrunning.sh](./helpers/dcrunning.sh)

### Starting servicetray at startup

For Ubuntu Linux, I did the following:

 * Presss the `<Super>` key and launch `Startup Applications`
 * My looked like this: `servicetray -config /home/am/.config/servicetray/vpns.yml`
 * Note that the PATH variable is going to be minimal. Either move your servicetray binary to `/usr/local/bin`, or specify the full path.

# TODO

 * Set up goreleaser to release binaries, debs etc.
 * Maybe include some icons.
 * Maybe add some more utilities:
   * add 'recipes'
   * cross-platform utilites (like, a cross-platform `dcrunning` binary)
 * Maybe add support for 'info/stats' for each item
