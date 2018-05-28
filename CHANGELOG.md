## 0.0.6

IMPROVEMENTS:
* Add ECS cluster draining when migrating autoscaling instances
* Ability to drain more than one instance at a time
* Drain and terminate instances in parallel
* Add option to set the minimum percent of instances to keep healthy

## 0.0.5

IMPROVEMENTS:
 * New auth parameter to set MFA token code instead of reading from prompt: --token-code
 * New auth parameter to set AWS profile: --aws-profile
 * You can now set some parameters from ENV vars (see help command)
 * Basic tests were added

## 0.0.4

BUG FIXES:
 * Fix the wrapper script to process script arguments properly: use `exec env` instead of `eval`

## 0.0.3

IMPROVEMENTS:
 * Print done when the AS migration finished
 * Change sleep from 20 to 10 secs

BUG FIXES:
 * When assuming roles wrong profile name was used in the script file
 * Include AWS_SECURITY_TOKEN in the generated env file (e.g. boto needs it..)

## 0.0.2

BUG FIXES:
 * Fix an issue where the user's home directory couldn't be determined

## 0.0.1

First release
