#Yeoman

Yeoman is an assistant to Earl. Namely it lets you manage the list of
enabled users.


##Interface
Yeoman is accessed over SSH. It listens on port 2022 by default. It presents
a readline-style interface, which allows for the addition and removal of keys.

###Commands

`log` Show the last N failed access attempts. This is useful for getting the RFID
serial number to add

`adduser` Add a new user

`deluser` Delete an existing user
