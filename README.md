# zssh
Ziti SSH is a project to replace `ssh` and `scp` with a more secure, zero-trust implementation 
of `ssh` and `scp`. 

These programs are not as feature rich as the ones provided by your operating system at this 
time, but we're looking for feedback. It's our assertion that these tools will cover 80% (or more) 
of your needs. If you find you are missing a favorite feature - please open an issue! We'd love to 
hear your feedback.

Read about CLI usage:
* zssh - [https://docs.openziti.io/blog/zitification/zitifying-ssh/](https://blog.openziti.io/zitifying-ssh)
* zscp - [https://docs.openziti.io/blog/zitification/zitifying-scp/](https://blog.openziti.io/zitifying-scp)

## Basic Usage
zssh:
```
   zssh -c identity.json <remoteUsername>@<targetIdentity> [flags]
```
zscp:
```
  zscp <remoteUsername>@<targetIdentity>:[Remote Path] [Local Path]
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
YOUR_EMAIL_ADDRESS=
```

#### Create Configs, Service, and Service Policies
```
ziti edge create config "${service_name}.host.v1" host.v1 \
  '{"protocol":"tcp", "address":"localhost","port":'"${the_port}"', "listenOptions": {"bindUsingEdgeIdentity":true}}'
# intercept is not needed for zscp/zssh but make it for testing if you like
ziti edge create config "${service_name}.intercept.v1" intercept.v1 \
  '{"protocols":["tcp"],"addresses":["'"${service_name}.ziti"'"], "portRanges":[{"low":'"${the_port}"', "high":'"${the_port}"'}]}'
ziti edge create service "${service_name}" \
  --configs "${service_name}.intercept.v1","${service_name}.host.v1"
ziti edge create service-policy "${service_name}-binding" Bind \
  --service-roles "@${service_name}" \
  --identity-roles "#${service_name}.binders" \
  --semantic "AnyOf"
ziti edge create service-policy "${service_name}-dialing" Dial \
  --service-roles "@${service_name}" \
  --identity-roles "#${service_name}.dialers" \
  --semantic "AnyOf"

# create two identities. one host - one client. Only necessary if you want/need them. 
# Skippable if you have identities already
ziti edge create identity "${server_identity}" \
  -a "${service_name}.binders" \
  -o "${server_identity}.jwt"
ziti edge create identity "${client_identity}" \
  -a "${service_name}.dialers" \
  -o "${client_identity}.jwt"

ziti edge enroll "${server_identity}.jwt"
ziti edge enroll "${client_identity}.jwt"
```

#### If Using OIDC for Secondary Auth

You can now use OIDC for secondary auth. This example will use Keycloak federated to GitHub and Google
* keycloak (or other OIDC server)
* know the audience your OIDC provider will inject in your JWTs and assign it to the 'aud' variable. For KeyCloak it 
  will be whatever the client is you make
* know the claim you plan to use that will be in the JWT returned from the OIDC provider, generally it'll be email 
  but it's not mandatory to use email
* create an identity in OpenZiti with an external-id matching the claim from above

```
ext_signer_name="keycloak-ext-jwt-signer"
iss="https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm"
jwks="https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm/protocol/openid-connect/certs"
aud="openziti-client"
claim="email"
auth_policy_name="keycloak_auth_policy"
YOUR_EMAIL_ADDRESS=

ext_jwt_signer_id=$(ziti edge create ext-jwt-signer "${ext_signer_name}" "$iss" -u "$jwks" -a "$aud" -c "$claim")
echo "External JWT signer created with id: $ext_jwt_signer_id"

keycloak_auth_policy=$(ziti edge create auth-policy "${auth_policy_name}" \
    --primary-cert-allowed \
    --primary-cert-expired-allowed \
    --secondary-req-ext-jwt-signer "${ext_jwt_signer_id}")
echo "keycloak_auth_policy created with id: ${keycloak_auth_policy}"

ziti edge update identity "${server_identity}" \
  --auth-policy "${keycloak_auth_policy}" \
  --external-id $YOUR_EMAIL_ADDRESS
  
./build/zssh \
  -s "${service_name}" \
  -i ~/.encrypted/.ssh/id_ed25519 \
  -c ./zsshClient.json \
  "${server_identity}"
```

#### OIDC for Primary Auth
```
auth_policy_name="keycloak_auth_policy_primary_auth"

# reuse the ext-jwt-signer
ext_jwt_signer_id=$(ziti edge list ext-jwt-signers 'name = "keycloak-ext-jwt-signer"' -j | jq -r .data[].id)
# or create a new one
ext_jwt_signer_id=$(ziti edge create ext-jwt-signer "${ext_signer_name}" "$iss" -u "$jwks" -a "$aud" -c "$claim")
echo "External JWT signer created with id: $ext_jwt_signer_id"

keycloak_auth_policy_primary=$(ziti edge create auth-policy "${auth_policy_name}" \
    --primary-ext-jwt-allowed \
    --primary-ext-jwt-allowed-signers "${ext_jwt_signer_id}")
echo "keycloak_auth_policy created with id: ${keycloak_auth_policy_primary}"

ziti edge update identity zsshSvcClient -P "${keycloak_auth_policy}"
ziti edge update identity zsshSvcClient --external-id $YOUR_EMAIL_ADDRESS
ziti edge update identity zsshSvcClient --auth-policy keycloak_auth_policy_primary
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
    -n openziti-client \
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
    -n openziti-client \
    -c ${identity_file} \
    ubuntu@zsshSvcServer \
    -- ls -l --color=auto
```

scp example:
```
./build/zscp \
    -i ${private_key} \
    -s "${service_name}" \
    -o \
    -a https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm \
    -n openziti-client \
    -c ${identity_file} \
    SECURITY.md \
    "${user_id}@${server_identity}":.
```

ssh remote command to verify SECURITY.md was transferred:
```
./build/zssh \
    -i ${private_key} \
    -s zsshSvc \
    -o \
    -a ${oidc_issuer} \
    -n openziti-client \
    -c ${identity_file} \
    ubuntu@zsshSvcServer \
    -- ls -l SECURITY.md
```

## Testing Locally
#### window 1
```
ziti edge quickstart
```

#### window 2
```
# make sure you're in the zssh project folder
ziti edge login localhost:1280 -u admin -p admin -y
service_name=zsshTest
client_identity="zsshClient"
server_identity="zsshServer"
the_port=22
YOUR_EMAIL_ADDRESS=clint.dovholuk@netfoundry.io

ziti edge create config "${service_name}.host.v1" host.v1 \
  '{"protocol":"tcp", "address":"localhost","port":'"${the_port}"', "listenOptions": {"bindUsingEdgeIdentity":true}}'
# intercept is not needed for zscp/zssh but make it for testing if you like
ziti edge create config "${service_name}.intercept.v1" intercept.v1 \
  '{"protocols":["tcp"],"addresses":["'"${service_name}.ziti"'"], "portRanges":[{"low":'"${the_port}"', "high":'"${the_port}"'}]}'
ziti edge create service "${service_name}" \
  --configs "${service_name}.intercept.v1","${service_name}.host.v1"
ziti edge create service-policy "${service_name}-binding" Bind \
  --service-roles "@${service_name}" \
  --identity-roles "#${service_name}.binders" \
  --semantic "AnyOf"
ziti edge create service-policy "${service_name}-dialing" Dial \
  --service-roles "@${service_name}" \
  --identity-roles "#${service_name}.dialers" \
  --semantic "AnyOf"

ext_signer_name="keycloak-ext-jwt-signer"
iss="https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm"
jwks="https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm/protocol/openid-connect/certs"
aud="openziti-client"
claim="email"
auth_policy_name="keycloak_auth_policy"

ext_jwt_signer_id=$(ziti edge create ext-jwt-signer "${ext_signer_name}" "$iss" -u "$jwks" -a "$aud" -c "$claim")
echo "External JWT signer created with id: $ext_jwt_signer_id"

identity_based_only=$(ziti edge create auth-policy "${auth_policy_name}-identity-based" \
    --primary-cert-allowed \
    --primary-cert-expired-allowed)
echo "identity_based_only created with id: ${identity_based_only}"

identity_and_oidc=$(ziti edge create auth-policy "${auth_policy_name}-identity-and-oidc" \
    --primary-cert-allowed \
    --primary-cert-expired-allowed \
    --secondary-req-ext-jwt-signer "${ext_jwt_signer_id}")
echo "identity_and_oidc created with id: ${identity_and_oidc}"

oidc_only=$(ziti edge create auth-policy "${auth_policy_name}-oidc-only" \
    --primary-ext-jwt-allowed \
    --primary-ext-jwt-allowed-signers "${ext_jwt_signer_id}")
echo "oidc_only created with id: ${oidc_only}"

# create and enroll two test identities
ziti edge create identity "${server_identity}" \
  -a "${service_name}.binders" \
  -o "${server_identity}.jwt"
ziti edge enroll "${server_identity}.jwt"

ziti edge create identity "${client_identity}" \
  -a "${service_name}.dialers" \
  -o "${client_identity}.jwt" \
  --external-id $YOUR_EMAIL_ADDRESS
ziti edge enroll "${client_identity}.jwt"

./ziti-edge-tunnel run-host -i "./${server_identity}.json"





































# A test identity using jwt authentication only
ziti edge create identity "${client_identity}-identity-only" \
  -a "${service_name}.dialers" \
  -o "${client_identity}.jwt"
ziti edge enroll "${client_identity}.jwt"


ziti edge update identity "${client_identity}" \
  --auth-policy "${keycloak_auth_policy}" \
  --external-id $YOUR_EMAIL_ADDRESS
  
```

#### window 3
```
# make sure you're in the zssh project folder
ziti edge login localhost:1280 -u admin -p admin -y
service_name=zsshTest
client_identity="zsshClient"
server_identity="zsshServer"
the_port=22
user_id="cd"
auth_policy_name="keycloak_auth_policy"

private_key="${HOME}/.encrypted/.ssh/id_ed25519"
oidc_issuer=https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm
identity_file="${PWD}/${client_identity}.json"


# login using the default policy
ziti edge update identity "${client_identity}" \
  --auth-policy "${auth_policy_name}-identity-based"
./build/zssh \
    -i "${private_key}" \
    -s "${service_name}" \
    -c "${identity_file}" \
    "${user_id}@${server_identity}"

# login using identity-based auth for primary and oidc for secondary
ziti edge update identity "${client_identity}" \
  --auth-policy "${auth_policy_name}-identity-and-oidc"
./build/zssh \
    -i "${private_key}" \
    -s "${service_name}" \
    -o \
    -a "${oidc_issuer}" \
    -n openziti-client \
    -c "${identity_file}" \
    -p 1234 \
    "${user_id}@${server_identity}"

# login using idp-based auth
ziti edge update identity "${client_identity}" \
  --auth-policy "${auth_policy_name}-oidc-only"
./build/zssh \
    -i "${private_key}" \
    -s "${service_name}" \
    -o \
    -a "${oidc_issuer}" \
    -n openziti-client \
    -p 1234 \
    --oidcOnly --controllerUrl https://localhost:1280 \
    "${user_id}@${server_identity}"










./build/zssh mfa enable \
  -c "${identity_file}" \
    -o \
    -a "${oidc_issuer}" \
    -n openziti-client \
    -p 1234





ziti edge update identity zsshSvcClient -P "${keycloak_auth_policy}"

```














---

extra stuff here
```
./build/zssh \
  -s "${service_name}" \
  -i ~/.encrypted/.ssh/id_ed25519 \
  -c "./${client_identity}.json" \
  "${server_identity}"
  
  
./build/zssh -c ./zsshClient.json "${server_identity}" -s zsshTest -i ~/.encrypted/.ssh/id_ed25519

# enable mfa
./build/zssh mfa enable -c "${identity_file}"
./build/zssh mfa remove -c "${identity_file}"




./build/zssh \
    mfa enable \
    -o \
    -a "${oidc_issuer}" \
    -n openziti-client \
    -c "${identity_file}"



./build/zssh \
    mfa remove \
    -o \
    -a "${oidc_issuer}" \
    -n openziti-client \
    -c "${identity_file}"


```








