# Molehill - a configurable ssh server for TCP port forwarding

I guess everybody knows the problem, you want to reach a resource behind a firewall, or need to fix up a device in a customers network, or need to connect to any other tcp port somewhere in the world. I would usually solve the issue for me by setting up an ssh serevr somewhere. But this always means that 2 parties (you and the network device that has access to your target) will need to have permissions to connect to the server, and if you are not carefull with config they could still get a shell, also it is annoying to setup the same config over and over again....

So I created molehill, a ssh server written in go, that only supports tcp forwards and nothing else. Additionally it uses socket files (unix domain sockets) to "store" all port connections while they are active so there are less possible clashes because of a certain port number being used twice. You can easily controll access via a simple yaml config file, and all during runtime (no need to restart molehill when adding clients)


## Server Usage

Run the server like any other go binary

```
go build .
./molehill
```

When the server starts for the first time it will create a config.yml file in the current directory. This file can be edited and will automatically be reloaded on save, no need to restart the server


### Running molehill in the background

If you want to run molehill as a service on your server please use systemd

TODO: Provide a simple systemd service script


### Client Usage:

Just use the regular SSH Client on your computer

Connect to a resource on the server

ssh -N -L 9998:localhost:1 lukas@localhost -p 2222

Publish a resource to the server

ssh -N -R 1:127.0.0.1:8000 lukas@localhost -p 2222


# Future: Mole client

use `go install github.com/s00500/molehill/cmd/mole@master`

TO BE DONE

This should also work well with: https://github.com/ferama/rospo

# Donate

If you like this project you can [buy me a coffee here](https://paypal.me/lukasbachschwell/5)

If you have any questions, ideas, improvements, or obvious bugs please open an issue or submit a PR.

# TODO: 

- [ ] log all connections, ignore forwarded port, ip-port -> filename
- [ ] Eventually allow to accept a new connection using a promt that is active for 30 seconds (It could also be email or telegram if you want :-))
- [ ] finally setup 2 mole commands to do client operation simpler
- [ ] Make sure the store lib knows yml and yaml!



