Usage:

Connect to a resource on the server

ssh -N -L 9998:localhost:1 lukas@localhost -p 2222

Publish a resource to the server

ssh -N -R 1:127.0.0.1:8000 lukas@localhost -p 2222




TODO: 

log all connections, ignore forwarded port, ip-port -> filename

Allow to accept a new connection using a promt that is active for 30 seconds
It could also be email or telegram if you want :-)
Only allow the check after password auth succeeded, 
then wait for check and only then complete the login

finally setup 2 mole commands to do it simpler

Ideally also we would NOT need to bind to any port !!! but use io copy internally... or something like that

Also: do not allow binding to any other than localhost and 127.0.0.1


Make sure the store lib knows yml and yaml!
