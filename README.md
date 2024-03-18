# zssh
Ziti SSH is a project to replace `ssh` and `scp` with a more secure, zero-trust implementation of `ssh` and `scp`.

These programs are not as feature rich as the ones provided by your operating system at this time but we're looking for feedback. It's our assertion that these tools will cover 80% (or more) of your needs. If you find you are missing a favorite feature - please open an issue! We'd love to hear your feedback

Read about:
* zssh - [https://docs.openziti.io/blog/zitification/zitifying-ssh/](https://blog.openziti.io/zitifying-ssh)
* zscp - [https://docs.openziti.io/blog/zitification/zitifying-scp/](https://blog.openziti.io/zitifying-scp)

## zssh usage
```
Error: accepts 1 arg(s), received 0
Usage:
   <remoteUsername>@<targetIdentity> [flags]

Flags:
  -i, --SshKeyPath string   Path to ssh key. default: $HOME/.ssh/id_rsa
  -c, --ZConfig string      Path to ziti config file. default: $HOME/.ziti/zssh.json
  -d, --debug               pass to enable additional debug information
  -h, --help                help for this command
  -s, --service string      service name. default: zssh (default "zssh")
```

## zscp usage
```
Usage:
  Remote to Local: zscp <remoteUsername>@<targetIdentity>:[Remote Path] [Local Path]
Local to Remote: zscp [Local Path][...] <remoteUsername>@<targetIdentity>:[Remote Path] [flags]

Flags:
  -i, --SshKeyPath string   Path to ssh key. default: $HOME/.ssh/id_rsa
  -c, --ZConfig string      Path to ziti config file. default: $HOME/.ziti/zssh.json
  -d, --debug               pass to enable additional debug information
  -h, --help                help for Remote
  -r, --recursive           pass to enable recursive file transfer
  -s, --service string      service name. default: zssh (default "zssh")
```

## zssh/zscp Quickstart

Here's a quick set of steps you can run to make two test identities, the configs, service and policies to enable zssh/zscp

```
# establish some variables which are used below
service_name=zsshSvc
client_identity="${service_name}Client"
server_identity="${service_name}Server"
the_port=22

# create two identities. one host - one client. Only necessary if you want/need them. Skippable if you
# already have an identity. provided here to just 'make it easy' to test/try
ziti edge create identity device "${server_identity}" -a "${service_name}.binders" -o "${server_identity}.jwt"
ziti edge create identity device "${client_identity}" -a "${service_name}.dialers" -o "${client_identity}.jwt"

# if you want to modify anything, often deleting the configs/services is easier than updating them
# it's easier to delete all the items too - so until you understand exactly how ziti works,
# make sure you clean them all up before making a change
ziti edge delete config "${service_name}.host.v1"
ziti edge delete config "${service_name}.intercept.v1"
ziti edge delete service "${service_name}"
ziti edge delete service-policy "${service_name}-binding"
ziti edge delete service-policy "${service_name}-dialing"

ziti edge create config "${service_name}.host.v1" host.v1 '{"protocol":"tcp", "address":"localhost","port":'"${the_port}"', "listenOptions": {"bindUsingEdgeIdentity":true}}'
# intercept is not needed for zscp/zssh but make it for testing if you like
ziti edge create config "${service_name}.intercept.v1" intercept.v1 '{"protocols":["tcp"],"addresses":["'"${service_name}.ziti"'"], "portRanges":[{"low":'"${the_port}"', "high":'"${the_port}"'}]}'
ziti edge create service "${service_name}" --configs "${service_name}.intercept.v1","${service_name}.host.v1"
ziti edge create service-policy "${service_name}-binding" Bind --service-roles "@${service_name}" --identity-roles "#${service_name}.binders"
ziti edge create service-policy "${service_name}-dialing" Dial --service-roles "@${service_name}" --identity-roles "#${service_name}.dialers"
```

If you no longer want these services and identities (i.e. you're cleaning up) run this or something like it:
```
ziti edge delete service-policy where 'name contains "'${service_name}'"'
ziti edge delete service where 'name contains "'${service_name}'"'
ziti edge delete config where 'name contains "'${service_name}'"'
ziti edge delete identity where 'name contains "'${service_name}'"'
```
