# servicetray

Configurable system tray widget for a group of related services

## Installation

servicetray is a go app. It's built with [https://github.com/getlantern/systray](systray) - see that repo for installation requirements

## Usage

Create a config file called `servicetray.yml`, before running `servicetray`

### Simple example

```
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

### Full example:

Typically you'd group multiple similar items together.

This example uses a 'template' for managing multiple vpns, via systemctl ...

```
title: VPNs
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


##
