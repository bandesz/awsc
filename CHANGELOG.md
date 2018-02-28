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
