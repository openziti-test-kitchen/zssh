# zssh
Ziti SSH is a project to replace `ssh` and `scp` with a more secure, zero-trust implementation of `ssh` and `scp`.

These programs are not as feature rich as the ones provided by your operating system at this time but we're looking for feedback. It's our assertion that these tools will cover 80% (or more) of your needs. If you find you are missing a favorite feature - please open an issue! We'd love to hear your feedback

Read about:
* zssh - https://ziti.dev/blog/zitifying-ssh/
* zscp - https://ziti.dev/blog/zitifying-scp

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
