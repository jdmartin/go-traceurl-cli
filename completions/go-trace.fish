set -l gotrace_commands
complete -f -c go-trace -n "not __fish_seen_subcommand_from $gotrace_commands" -a "-h" -d 'Shows the help'
complete -f -c go-trace -n "not __fish_seen_subcommand_from $gotrace_commands" -a "--help" -d 'Shows the help'
complete -f -c go-trace -n "not __fish_seen_subcommand_from $gotrace_commands" -a "-j" -d 'Outputs results as JSON'
complete -f -c go-trace -n "not __fish_seen_subcommand_from $gotrace_commands" -a "-s" -d 'Outputs only the final/clean URL'
complete -f -c go-trace -n "not __fish_seen_subcommand_from $gotrace_commands" -a "-v" -d 'Shows all results in tabular format'
complete -f -c go-trace -n "not __fish_seen_subcommand_from $gotrace_commands" -a "-w" -d 'Sets the width of the URL column when using -v. (Ex: -w 120)'
