# zssh
Ziti SSH is a project to replace `ssh` and `scp` with a more secure, zero-trust implementation 
of `ssh` and `scp`. 

These programs are not as feature rich as the ones provided by your operating system at this 
time, but we're looking for feedback. It's our assertion that these tools will cover 80% (or more) 
of your needs. If you find you are missing a favorite feature - please open an issue! We'd love to 
hear your feedback.

Read about:
* zssh - [https://blog.openziti.io/zitifying-scp](https://blog.openziti.io/zitifying-ssh)
* zscp - [https://blog.openziti.io/zitifying-scp](https://blog.openziti.io/zitifying-scp)

Explore the CLI yourself, or look through the CLI commands online to explore usage etc.
* [zssh usage](./docs/zssh/zssh.md)
* [zscp usage](./docs/zscp/zscp.md)

## Prerequisites - Configuring the Overlay

The steps below will show you how to test/use `zssh` or `zscp` entirely locally. The steps rely on using the 
`ziti edge quickstart` command to start an overlay network that's usable from localhost only. The steps will work 
fine with overlay networks deployed through other mechanisms as well, you will simply need to adjust the parameters 
accordingly. By default, as shown below, these values will expect you are running the quickstart and using localhost:

    ZITI_USER="admin"
    ZITI_PWD="admin"
    ZITI_CONTROLLER="localhost:1280"

The overlay will be configured with three different authentication policies allows for authentication by 
certificate, by certificate with secondary OIDC, and with OIDC-only auth.

### Open Three Terminals

It will be easiest to start three separate local terminal windows/panes. Open three different windows now:
* where `ziti edge quickstart` runs
* a "server" terminal where `ziti-edge-tunnel` will run and will provide offload for `zssh/zscp`
* a "client" terminal where you'll run `zssh/zscp`

### Start OpenZiti

In the first terminal window, start an OpenZiti overlay network. Ensure the `ziti` binary is on your path, or 
provide the full path to the binary and start it with the `edge quickstart` command. For example, if `ziti` is on 
your path, you can simply run:

    ziti edge quickstart

It should take no more than 10 seconds (usually much less) for the overlay network to come online. It will make an 
ephemeral OpenZiti environment for you to use and play with. It is _not_ permanent. Read the `--help` for the `ziti 
edge quickstart` command for more details. Since the `--home` parameter was not supplied, when it is stopped 
normally using `ctrl+c`, the process will remove the environment. If you wish to start over, stop the 
OpenZiti environment, then start it back up again and rerun the steps.

### Establish Some Variables To Use With Sample Commands

The commands below for the quickstart will rely on variables being set in your shell. Notably, you need the name of 
a service, an identity that will serve as the target/server of `zssh/zscp`, and a client identity. Port 22 will be 
assumed as this is the default ssh port. If you plan to use OIDC-based authentication, you'll need to ensure your 
email address is set into `YOUR_EMAIL_ADDRESSS` and you'll need to ensure the JWT returned from the IdP has a claim 
named `email` and it's the same values what was set into `YOUR_EMAIL_ADDRESSS`. If not using OIDC, this is of course 
optional.

Set the following variables in the "server" terminal instance, as well as the "client" terminal instance. If a command
shown below fails, it's likely these variables are not set. Make sure you set them. This quickstart will create a 
service named `zsshSvc` and will use the name of the service as a prefix for all other entities created in the 
OpenZiti overlay network, hopefully making them easy to find if needed.  

    # establish some variables which are used below
    service_name=zsshSvc
    client_identity="${service_name}Client"
    server_identity="${service_name}Server"
    the_port=22
    YOUR_EMAIL_ADDRESS=

    ext_signer_name="keycloak-ext-jwt-signer"
    oidc_issuer="https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm"
    jwks="https://keycloak.clint.demo.openziti.org:8446/realms/zitirealm/protocol/openid-connect/certs"
    aud="openziti-client"
    claim="email"
    auth_policy_name="keycloak_auth_policy"
    private_key=
    user_id="$USER" #use the real remote user id here

#### Create Configs, Service, and Service Policies

With the variables shown above set, in the "server" terminal execute the following commands and ensure they all 
succeed. If necessary, you can always start over by stopping the `ziti edge quickstart` as described above. As 
shown, these commands will create a service, an `intercept.v1` and `host.v1` config, and two service policies 
authorizing the identities to `dial` and `bind` the service.

    ziti edge create config "${service_name}.host.v1" host.v1 \
      '{"protocol":"tcp", "address":"localhost","port":'"${the_port}"', "listenOptions": {"bindUsingEdgeIdentity":true}}'
    # intercept is not needed for zscp/zssh but make it for testing if you like
    ziti edge create config "${service_name}.intercept.v1" intercept.v1 \
      '{"protocols":["tcp"],"addresses":["'"${service_name}.ziti"'"], "portRanges":[{"low":'"${the_port}"', "high":'"${the_port}"'}]}'
    ziti edge create service "${service_name}" \
      --configs "${service_name}.intercept.v1","${service_name}.host.v1"
    ziti edge create service-policy "${service_name}-bind" Bind \
      --service-roles "@${service_name}" \
      --identity-roles "#${service_name}.binders" \
      --semantic "AnyOf"
    ziti edge create service-policy "${service_name}-dial" Dial \
      --service-roles "@${service_name}" \
      --identity-roles "#${service_name}.dialers" \
      --semantic "AnyOf"

### Create an External JWT Signer and Auth Policies

The following commands will create an External JWT signer and use that signer with the three different expected auth 
policies: certificate, certificate with secondary OIDC, OIDC only:

    ext_jwt_signer_id=$(ziti edge create ext-jwt-signer "${service_name}.${ext_signer_name}" "$oidc_issuer" -u "$jwks" -a "$aud" -c "$claim")
    echo "External JWT signer created with id: $ext_jwt_signer_id"
    
    identity_based_only=$(ziti edge create auth-policy "${service_name}.${auth_policy_name}-identity-based" \
    --primary-cert-allowed \
    --primary-cert-expired-allowed)
    echo "identity_based_only created with id: ${identity_based_only}"
    
    identity_and_oidc=$(ziti edge create auth-policy "${service_name}.${auth_policy_name}-identity-and-oidc" \
    --primary-cert-allowed \
    --primary-cert-expired-allowed \
    --secondary-req-ext-jwt-signer "${ext_jwt_signer_id}")
    echo "identity_and_oidc created with id: ${identity_and_oidc}"
    
    oidc_only=$(ziti edge create auth-policy "${service_name}.${auth_policy_name}-oidc-only" \
    --primary-ext-jwt-allowed \
    --primary-ext-jwt-allowed-signers "${ext_jwt_signer_id}")
    echo "oidc_only created with id: ${oidc_only}"

### Create the Necessary Identities

With the service created and authorized, two identities will be necessary. One identity will bind the ssh service
and the other identity will be used to dial the service and connect to the sshd service.

    # create two identities, one to host sshd - one to connect to sshd
    ziti edge create identity "${server_identity}" \
    -a "${service_name}.binders" \
    -o "${server_identity}.jwt"
    ziti edge enroll "${server_identity}.jwt"
    
    ziti edge create identity "${client_identity}" \
    -a "${service_name}.dialers" \
    -o "${client_identity}.jwt" \
    --external-id $YOUR_EMAIL_ADDRESS
    ziti edge enroll "${client_identity}.jwt"

### Run the sshd-server Identity

Download and run the `ziti-edge-tunnel` binary from GitHub. You can find the URL for the latest `ziti-edge-tunnel`
by going to https://github.com/openziti/ziti-tunnel-sdk-c/releases/latest. Download the distribution, and unzip it.
For example, if you are using linux you might run:

    wget https://github.com/openziti/ziti-tunnel-sdk-c/releases/download/v1.1.3/ziti-edge-tunnel-Linux_x86_64.zip
    unzip ziti-edge-tunnel-Linux_x86_64.zip

With the `ziti-edge-tunnel` executable downloaded, execute it to provide an identity providing access to `sshd`.
This identity will remain running for the duration of testing:

    ./ziti-edge-tunnel run-host -i "./${server_identity}.json"

## Using zssh/zscp

With the OpenZiti overlay quickstart running and configured, you can now use `zssh` or `zscp` in one of three ways:
* identity-based (certificate) authentication
* identity-based (certificate) authentication with secondary OIDC authentication
* OIDC authentication only

In these examples, the identity binding sshd will always use certificate-based authentication. Only the
identity running `zssh/zscp` will change the authentication mechanism.

### Identity-based (certificate) Authentication

    # login using the default policy
    ziti edge update identity "${client_identity}" \
      --auth-policy "${auth_policy_name}-identity-based"
    ./build/zssh \
      -i "${private_key}" \
      -s "${service_name}" \
      -c "${client_identity}.json" \
      "${user_id}@${server_identity}"

### Identity-based (certificate) Authentication With Secondary OIDC Authentication

You can use OIDC for secondary auth along with certificate-based authentication. For example, you can federate your
Keycloak IdP to GitHub, Google, etc. but as long as the identity is returned with the proper claim (email) and an
identity is mapped to the cliam using an external id, secondary auth will succeed.
* keycloak (or other OIDC server)
* know the audience your OIDC provider will inject in your JWTs and assign it to the 'aud' variable. For KeyCloak it
  will be whatever the client is you make
* know the claim you plan to use that will be in the JWT returned from the OIDC provider, generally it'll be email
  but it's not mandatory to use email
* create an identity in OpenZiti with an external-id matching the claim from above


      # login using identity-based auth for primary and oidc for secondary
      ziti edge update identity "${client_identity}" \
        --auth-policy "${auth_policy_name}-identity-and-oidc"
      ./build/zssh \
        -i "${private_key}" \
        -s "${service_name}" \
        -o \
        -a "${oidc_issuer}" \
        -n openziti-client \
        -c "${client_identity}.json" \
        -p 1234 \
        "${user_id}@${server_identity}"


### OIDC Authentication Only

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
      --oidcOnly \
      --controllerUrl https://localhost:1280 \
      "${user_id}@${server_identity}"

### Manual Cleanup

If for some reason you don't want to tear down your OpenZiti overlay, you can run these commands to clean up the:
* two configs
* one service
* two service policies
* two identities
* three auth policies
* one external jwt signer


      ziti edge delete configs where 'name contains "'"${service_name}"'"'
      ziti edge delete service where 'name contains "'"${service_name}"'"'
      ziti edge delete service-policies where 'name contains "'"${service_name}"'"'
      ziti edge delete identities where 'name contains "'"${service_name}"'"'
      ziti edge delete auth-policies where 'name contains "'"${service_name}"'"'
      ziti edge delete ext-jwt-signer where 'name contains "'"${service_name}"'"'

## Adding TOTP

The `zssh` and `zscp` binaries will support using time-based, one-time passcodes (TOTP) as yet another layer of
authentication. Pass the proper params to the `mfa enable` or `mfa remove` to add/remove a TOTP requirement.

Example using client-based auth:

    ziti edge update identity "${client_identity}" \
      --auth-policy "${auth_policy_name}-identity-and-oidc"
    ./build/zssh mfa enable \
      -o \
      -a "${oidc_issuer}" \
      -n openziti-client \
      -c "${client_identity}.json" \
      -p 1234
    
    ./build/zssh mfa remove \
      -o \
      -a "${oidc_issuer}" \
      -n openziti-client \
      -c "${client_identity}.json" \
      -p 1234
    
    ./build/zssh \
      -i "${private_key}" \
      -s "${service_name}" \
      -o \
      -a "${oidc_issuer}" \
      -n openziti-client \
      -c "${client_identity}.json" \
      -p 1234 \
      "${user_id}@${server_identity}"

## Other Examples

scp example:

    # echo make a file to transfer
    echo "a" > a.txt
    
    # scp the a.txt file as b.txt
    ./build/zscp \
      -i "${private_key}" \
      -s "${service_name}" \
      -o \
      -a "${oidc_issuer}" \
      -n openziti-client \
      -c "${client_identity}.json" \
      -p 1234 \
      a.txt "${user_id}@${server_identity}":./b.txt

Use zssh and a remote command to verify b.txt was transferred and contains the proper contents:

    ./build/zssh \
      -i "${private_key}" \
      -s "${service_name}" \
      -o \
      -a "${oidc_issuer}" \
      -n openziti-client \
      -c "${client_identity}.json" \
      -p 1234 \
      "${user_id}@${server_identity}" -- cat ./b.txt





