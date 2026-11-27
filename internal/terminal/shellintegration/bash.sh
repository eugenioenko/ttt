# ttt bash shell integration, loaded with --rcfile in place of ~/.bashrc.
if [ -r ~/.bashrc ]; then
	. ~/.bashrc
fi

__ttt_prompt_mark() {
	case $PS1 in
	*'\e]133;A\a'*) ;;
	*) PS1='\[\e]133;A\a\]'$PS1 ;;
	esac
	case ${PS0-} in
	*'\e]133;C\a'*) ;;
	*) PS0=${PS0-}'\e]133;C\a' ;;
	esac
}

# Without PS0 (bash < 4.4) nothing would end the prompt mark when a command
# starts, and a resize would blank the command's output.
if ((BASH_VERSINFO[0] > 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] >= 4))); then
	# Runs last, so prompts that PROMPT_COMMAND rebuilds keep their mark.
	if [[ $(declare -p PROMPT_COMMAND 2>/dev/null) == "declare -a"* ]]; then
		PROMPT_COMMAND+=(__ttt_prompt_mark)
	else
		PROMPT_COMMAND=${PROMPT_COMMAND:+$PROMPT_COMMAND$'\n'}__ttt_prompt_mark
	fi
fi
