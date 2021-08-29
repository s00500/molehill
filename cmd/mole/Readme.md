

Gameplan:

Read and write config file to ~/.config/mole

mole -l lists available configs

mole -c creates new config

mole -c clears config

mole "configname" connects config
mole "configname" "configname" "configname" connects multiple configs


mole -c:

ask configname

1 config can have multiple things

ask if this is a publish or grab with select
ask server and port (default to the one from config 1 if available)

ask port and host
ask server side reference (default to configname:1)
ask to autoreconnect

ask to add more entries to this config
