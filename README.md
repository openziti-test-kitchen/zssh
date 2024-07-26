# zssh
Ziti SSH is a project to replace `ssh` and `scp` with a more secure, zero-trust implementation 
of `ssh` and `scp`. 

These programs are not as feature rich as the ones provided by your operating system at this 
time, but we're looking for feedback. It's our assertion that these tools will cover 80% (or more) 
of your needs. If you find you are missing a favorite feature - please open an issue! We'd love to 
hear your feedback.

Read about:
* zssh - [https://docs.openziti.io/blog/zitification/zitifying-ssh/](https://blog.openziti.io/zitifying-ssh)
* zscp - [https://docs.openziti.io/blog/zitification/zitifying-scp/](https://blog.openziti.io/zitifying-scp)

## zssh usage
```
Error: accepts 1 arg(s), received 0
Usage:
   <remoteUsername>@<targetIdentity> [flags]

Flags:
  -p, --CallbackPort string   Port for Callback. default: 63275 (default "63275")
  -n, --ClientID string       IdP ClientID. default: cid1 (default "cid1")
  -e, --ClientSecret string   IdP ClientSecret. default: (empty string - use PKCE)
  -a, --OIDCIssuer string     URL of the OpenID Connect provider. required (default "https://dev-yourid.okta.com")
  -i, --SshKeyPath string     Path to ssh key. default: $HOME/.ssh/id_rsa
  -c, --ZConfig string        Path to ziti config file. default: $HOME/.ziti/zssh.json
  -d, --debug                 pass to enable additional debug information
  -h, --help                  help for this command
  -o, --oidc                  toggle OIDC mode. default: false
  -s, --service string        service name. default: zssh (default "zssh")
```

## zscp usage
```
Usage:
  Remote to Local: zscp <remoteUsername>@<targetIdentity>:[Remote Path] [Local Path]
Local to Remote: zscp [Local Path][...] <remoteUsername>@<targetIdentity>:[Remote Path] [flags]

Flags:
  -p, --CallbackPort string   Port for Callback. default: 63275 (default "63275")
  -n, --ClientID string       IdP ClientID. default: cid1 (default "cid1")
  -e, --ClientSecret string   IdP ClientSecret. default: (empty string - use PKCE)
  -a, --OIDCIssuer string     URL of the OpenID Connect provider. required (default "https://dev-yourid.okta.com")
  -i, --SshKeyPath string     Path to ssh key. default: $HOME/.ssh/id_rsa
  -c, --ZConfig string        Path to ziti config file. default: $HOME/.ziti/zssh.json
  -d, --debug                 pass to enable additional debug information
  -h, --help                  help for Remote
  -o, --oidc                  toggle OIDC mode. default: false
  -r, --recursive             pass to enable recursive file transfer
  -s, --service string        service name. default: zssh (default "zssh")
```

## zssh/zscp Quickstart

Here's a quick set of steps you can run to make two test identities, the configs, service and policies to enable zssh/zscp

#### Establish Some Variables To Use With Sample Commands
```
# establish some variables which are used below
service_name=zsshSvc
client_identity="${service_name}Client"
server_identity="${service_name}Server"
the_port=22
```

#### Create Configs, Service, and Service Policies
```
ziti edge create config "${service_name}.host.v1" host.v1 '{"protocol":"tcp", "address":"localhost","port":'"${the_port}"', "listenOptions": {"bindUsingEdgeIdentity":true}}'
# intercept is not needed for zscp/zssh but make it for testing if you like
ziti edge create config "${service_name}.intercept.v1" intercept.v1 '{"protocols":["tcp"],"addresses":["'"${service_name}.ziti"'"], "portRanges":[{"low":'"${the_port}"', "high":'"${the_port}"'}]}'
ziti edge create service "${service_name}" --configs "${service_name}.intercept.v1","${service_name}.host.v1"
ziti edge create service-policy "${service_name}-binding" Bind --service-roles "@${service_name}" --identity-roles "#${service_name}.binders" --semantic "AnyOf"
ziti edge create service-policy "${service_name}-dialing" Dial --service-roles "@${service_name}" --identity-roles "#${service_name}.dialers" --semantic "AnyOf"

# create two identities. one host - one client. Only necessary if you want/need them. Skippable if you have identities already
ziti edge create identity "${server_identity}" -a "${service_name}.binders" -o "${server_identity}.jwt"
ziti edge create identity "${client_identity}" -a "${service_name}.dialers" -o "${client_identity}.jwt"

ziti edge enroll "${server_identity}.jwt"
ziti edge enroll "${client_identity}.jwt"
```

#### IF Using OIDC for Secondary Auth

You can now use OIDC for secondary auth. This example will use Keycloak federated to GitHub and Google
* keycloak (or other OIDC server)
* know the audience your OIDC provider will inject in your JWTs and assign it to the 'aud' variable. For KeyCloak it 
  will be whatever the client is you make
* know the claim you plan to use that will be in the JWT returned from the OIDC provider, generally it'll be email 
  but it's not mandatory to use email
* create an identity in OpenZiti with an external-id matching the claim from above

```
YOUR_EMAIL_ADDRESS=
ext_signer_name="keycloak-ext-jwt-signer"
iss="https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm"
jwks="https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm/protocol/openid-connect/certs"
aud="cid1"
claim="email"
auth_policy_name="keycloak_auth_policy"

ext_jwt_signer_id=$(ziti edge create ext-jwt-signer "${ext_signer_name}" "$iss" -u "$jwks" -a "$aud" -c "$claim")
echo "External JWT signer created with id: $ext_jwt_signer_id"

keycloak_auth_policy=$(ziti edge create auth-policy "${auth_policy_name}" \
    --primary-cert-allowed \
    --primary-cert-expired-allowed \
    --secondary-req-ext-jwt-signer "${ext_jwt_signer_id}")
echo "keycloak_auth_policy created with id: ${keycloak_auth_policy}"

ziti edge update identity zsshSvcClient -P "${keycloak_auth_policy}"
ziti edge update identity zsshSvcClient --external-id $YOUR_EMAIL_ADDRESS
```



#### Clean Up and Start Again
```
# already have an identity. provided here to just 'make it easy' to test/try
ziti edge delete identity zsshSvcServer
ziti edge delete identity zsshSvcClient

ziti edge delete auth-policy keycloak_auth_policy
ziti edge delete ext-jwt-signer "${ext_signer_name}"

# if you want to modify anything, often deleting the configs/services is easier than updating them
# it's easier to delete all the items too - so until you understand exactly how ziti works,
# make sure you clean them all up before making a change
ziti edge delete config "${service_name}.host.v1"
ziti edge delete config "${service_name}.intercept.v1"
ziti edge delete service "${service_name}"
ziti edge delete service-policy "${service_name}-binding"
ziti edge delete service-policy "${service_name}-dialing"
```

If you no longer want these services and identities (i.e. you're cleaning up) run this or something like it:

## Examples

variables established:
```
private_key=~/.ssh/id_rsa
oidc_issuer=https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm
identity_file=/home/cd/git/github/openziti-test-kitchen/zssh/zsshSvcClient.json
```

ssh example:
```
./build/zssh \
    -i ${private_key} \
    -s zsshSvc \
    -o \
    -a ${oidc_issuer} \
    -n cid1 \
    -c ${identity_file} \
    ubuntu@zsshSvcServer
```

remote command execution ssh example. NOTE the use of `--` to denote where the
command starts and the `zssh` flags end is important! this command will list
the contents of the remote connection with colorized results:
```
./build/zssh \
    -i ${private_key} \
    -s zsshSvc \
    -o \
    -a ${oidc_issuer} \
    -n cid1 \
    -c ${identity_file} \
    ubuntu@zsshSvcServer \
    -- ls -l --color=auto
```

scp example:
```
./build/zscp \
    -i ${private_key} \
    -s zsshSvc \
    -o \
    -a https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm \
    -n cid1 \
    -c ${identity_file} \
    SECURITY.md \
    ubuntu@zsshSvcServer:.
```

ssh remote command to verify SECURITY.md was transferred:
```
./build/zssh \
    -i ${private_key} \
    -s zsshSvc \
    -o \
    -a ${oidc_issuer} \
    -n cid1 \
    -c ${identity_file} \
    ubuntu@zsshSvcServer \
    -- ls -l SECURITY.md
```

## Testing Locally
#### window 1
```
ziti egde quickstart
```

#### window 2
```
ziti edge login localhost:1280 -u admin -p admin -y
service_name=zsshTest
client_identity="zsshClient"
server_identity="zsshServer"
the_port=22
ziti edge create config "${service_name}.host.v1" host.v1 \
'{"protocol":"tcp", "address":"localhost", "port":'"$the_port}"', "listenOptions": {"bindUsingEdgeIdentity":true}}'
ziti edge create service "${service_name}" --configs "${service_name}.host.v1"
ziti edge create service-policy "${service_name}-binding" Bind --service-roles "@${service_name}" --identity-roles ${service_name}.binders" --semantic "AnyOf"
ziti edge create service-policy "${service_name}-dialing" Dial --service-roles "@${service_name}" --identity-roles ${service_name}.dialers" --semantic "AnyOf"
ziti edge create identity "${server_identity}" -a "${service_name}.binders" -o "${server_identity}.jwt"
ziti edge create identity "${client_identity}" -a "${service_name}.dialers" -o "${client_identity}.jwt"
ziti edge enroll "${server_identity}.jwt"
ziti edge enroll "${client_identity}.jwt"

./ziti-edge-tunnel run-host -i ./zsshServer.json
```

#### window 3
```
git checkout add-config-support
mkdir build
go build -o build ./...
./build/zssh -i ./zsshClient.json zsshServer
```








