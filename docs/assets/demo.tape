# The README demo: from a Magento project to a running store.
#
# Record it with VHS (https://github.com/charmbracelet/vhs) from the root of this repository, with
# Docker running, hm on the PATH, and HM_DEMO_PROJECT naming a Magento 2 project that has a
# composer.lock and a database dump at ./dump.sql:
#
#   HM_DEMO_PROJECT=~/src/some-magento-project vhs docs/assets/demo.tape
#
# VHS writes the GIF relative to where it runs (docs/assets/demo.gif); the typed commands run in
# the project, entered off camera. --yes keeps hm setup from asking anything; the dump answers the
# one question that has no default (see docs/install.md).

Output docs/assets/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 640
Set Padding 20
Set TypingSpeed 50ms

Hide
Type 'cd "$HM_DEMO_PROJECT" && clear'
Enter
Show

Type "hm --version"
Enter
Sleep 2s

Type "hm setup --yes --domain=demo.local --db-dump=./dump.sql"
Enter
Sleep 20s

Type "hm start"
Enter
Sleep 15s

Type "hm magento cache:clean"
Enter
Sleep 5s
