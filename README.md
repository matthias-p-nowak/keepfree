# keepfree
keeping a certain disk space free.

## Function

It periodically checks the free space for certain directories. One can specify which directories should be cleaned and how much free space should be made. It logs to syslog, which cutoff day has been used.

On receiving SigHUP it prints out the current file date histogram.
