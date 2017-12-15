# AWS Companion (awsc)

The AWS Companion app contains useful additions to the AWS cli.

## How to install

Go to https://github.com/opsidian/awsc/releases and download the appropriate binary for your system.
Rename it to ```awsc``` and move it somewhere on your PATH.

## Available commands

### Authenticate with MFA

```
AWS_PROFILE=my-profile awsc auth
```

The command will create temporary credentials and save them under ~/.awsc with the given expiration time.
The script generates three files:
 - ~/.awsc/my-profile.json: the temporary credentials in JSON format
 - ~/.awsc/my-profile.env: the credentials exported as environment variables, so you can source them from a bash script
 - ~/.awsc/my-profile: a helper script which sources ~/.aws/my-profile.env and runs the given command. It also runs awsc auth to automatically reauthenticate if necessary.

I suggest to add ~/.awsc to your PATH in your bash profile:

```
export PATH=$PATH:$HOME/.awsc
```

Example:
```
$ AWS_PROFILE=my-company-dev awsc auth
MFA token: ******
$ my-company-dev aws s3 list-buckets
```

If you plan to use the helper script, then you have to run the ```awsc auth``` command only once per profile.

If you want to use your own script then include this two lines before you interact with AWS:

```
AWS_PROFILE=my-company-dev awsc auth
. $HOME/.awsc/my-company-dev.env
```

### Replace all instances in an Auto Scaling group

```
awsc autoscaling migrate <auto scaling group name>
```

The command will terminate all auto scaling group instances one-by-one. When an instance is terminated it waits for a new instance to be created and be in service.

If your auto scaling group has only one instance then this command might cause downtime.
