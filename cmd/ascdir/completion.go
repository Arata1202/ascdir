package main

import (
	"errors"
	"fmt"
	"io"
)

func runCompletion(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: ascdir completion <bash|zsh|fish|powershell>")
	}
	var script string
	switch args[0] {
	case "bash":
		script = bashCompletion
	case "zsh":
		script = zshCompletion
	case "fish":
		script = fishCompletion
	case "powershell":
		script = powershellCompletion
	default:
		return fmt.Errorf("unsupported shell %q; expected bash, zsh, fish, or powershell", args[0])
	}
	_, err := io.WriteString(stdout, script)
	return err
}

const bashCompletion = `_ascdir() {
  local current previous
  current="${COMP_WORDS[COMP_CWORD]}"
  previous="${COMP_WORDS[COMP_CWORD-1]}"
  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=($(compgen -W "auth init pull push check price-points app-store testflight completion version help" -- "${current}"))
  elif [[ ${previous} == "auth" ]]; then
    COMPREPLY=($(compgen -W "login check logout" -- "${current}"))
  elif [[ ${previous} == "app-store" ]]; then
    COMPREPLY=($(compgen -W "status submit release" -- "${current}"))
  elif [[ ${previous} == "testflight" ]]; then
    COMPREPLY=($(compgen -W "status distribute" -- "${current}"))
  elif [[ ${previous} == "completion" ]]; then
    COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "${current}"))
  elif [[ ${current} == -* ]]; then
    case "${COMP_WORDS[1]}" in
      init) COMPREPLY=($(compgen -W "--bundle-id --version --platform --locale --config --force" -- "${current}")) ;;
      pull) COMPREPLY=($(compgen -W "--config --dry-run --allow-local-asset-deletions" -- "${current}")) ;;
      push) COMPREPLY=($(compgen -W "--config --dry-run --allow-empty --allow-irreversible --allow-asset-deletions --allow-availability-changes --allow-commercial-changes" -- "${current}")) ;;
      check) COMPREPLY=($(compgen -W "--config" -- "${current}")) ;;
      price-points) COMPREPLY=($(compgen -W "--config --territory" -- "${current}")) ;;
      app-store) COMPREPLY=($(compgen -W "--config --json --build --release-type --earliest-release-date --dry-run --confirm" -- "${current}")) ;;
      testflight) COMPREPLY=($(compgen -W "--config --json --group --build --dry-run --confirm" -- "${current}")) ;;
    esac
  fi
}
complete -F _ascdir ascdir
`

const zshCompletion = `#compdef ascdir

_ascdir() {
  local -a commands auth_commands app_store_commands status_commands shells
  commands=(auth init pull push check price-points app-store testflight completion version help)
  auth_commands=(login check logout)
	app_store_commands=(status submit release)
	status_commands=(status distribute)
  shells=(bash zsh fish powershell)
  if (( CURRENT == 2 )); then
    _describe 'command' commands
  elif [[ ${words[2]} == auth && CURRENT == 3 ]]; then
    _describe 'auth command' auth_commands
  elif [[ ${words[2]} == app-store && CURRENT == 3 ]]; then
	_describe 'app-store command' app_store_commands
  elif [[ ${words[2]} == testflight && CURRENT == 3 ]]; then
    _describe 'status command' status_commands
  elif [[ ${words[2]} == completion && CURRENT == 3 ]]; then
    _describe 'shell' shells
  else
    case ${words[2]} in
      init) _values 'option' --bundle-id --version --platform --locale --config --force ;;
      pull) _values 'option' --config --dry-run --allow-local-asset-deletions ;;
      push) _values 'option' --config --dry-run --allow-empty --allow-irreversible --allow-asset-deletions --allow-availability-changes --allow-commercial-changes ;;
      check) _values 'option' --config ;;
      price-points) _values 'option' --config --territory ;;
	  app-store) _values 'option' --config --json --build --release-type --earliest-release-date --dry-run --confirm ;;
	  testflight) _values 'option' --config --json --group --build --dry-run --confirm ;;
      *) _files ;;
    esac
  fi
}

compdef _ascdir ascdir
`

const fishCompletion = `complete -c ascdir -f
complete -c ascdir -n '__fish_use_subcommand' -a 'auth init pull push check price-points app-store testflight completion version help'
complete -c ascdir -n '__fish_seen_subcommand_from auth' -a 'login check logout'
complete -c ascdir -n '__fish_seen_subcommand_from app-store' -a 'status submit release'
complete -c ascdir -n '__fish_seen_subcommand_from testflight' -a 'status distribute'
complete -c ascdir -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish powershell'
complete -c ascdir -n '__fish_seen_subcommand_from init' -l bundle-id -r
complete -c ascdir -n '__fish_seen_subcommand_from init' -l version -r
complete -c ascdir -n '__fish_seen_subcommand_from init' -l platform -r -a 'IOS MAC_OS TV_OS VISION_OS'
complete -c ascdir -n '__fish_seen_subcommand_from init' -l locale -r
complete -c ascdir -n '__fish_seen_subcommand_from init pull push check' -l config -r
complete -c ascdir -n '__fish_seen_subcommand_from price-points' -l config -r
complete -c ascdir -n '__fish_seen_subcommand_from price-points' -l territory -r
complete -c ascdir -n '__fish_seen_subcommand_from app-store testflight' -l config -r
complete -c ascdir -n '__fish_seen_subcommand_from app-store testflight' -l json
complete -c ascdir -n '__fish_seen_subcommand_from testflight' -l group -r
complete -c ascdir -n '__fish_seen_subcommand_from testflight' -l build -r
complete -c ascdir -n '__fish_seen_subcommand_from testflight' -l dry-run
complete -c ascdir -n '__fish_seen_subcommand_from testflight' -l confirm -r
complete -c ascdir -n '__fish_seen_subcommand_from app-store' -l build -r
complete -c ascdir -n '__fish_seen_subcommand_from app-store' -l release-type -r -a 'MANUAL AFTER_APPROVAL SCHEDULED'
complete -c ascdir -n '__fish_seen_subcommand_from app-store' -l earliest-release-date -r
complete -c ascdir -n '__fish_seen_subcommand_from app-store' -l dry-run
complete -c ascdir -n '__fish_seen_subcommand_from app-store' -l confirm -r
complete -c ascdir -n '__fish_seen_subcommand_from init' -l force
complete -c ascdir -n '__fish_seen_subcommand_from pull push' -l dry-run
complete -c ascdir -n '__fish_seen_subcommand_from pull' -l allow-local-asset-deletions
complete -c ascdir -n '__fish_seen_subcommand_from push' -l allow-empty
complete -c ascdir -n '__fish_seen_subcommand_from push' -l allow-irreversible
complete -c ascdir -n '__fish_seen_subcommand_from push' -l allow-asset-deletions
complete -c ascdir -n '__fish_seen_subcommand_from push' -l allow-availability-changes
complete -c ascdir -n '__fish_seen_subcommand_from push' -l allow-commercial-changes
`

const powershellCompletion = `Register-ArgumentCompleter -Native -CommandName ascdir -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $elements = $commandAst.CommandElements
  $candidates = if ($elements.Count -le 2) {
    'auth','init','pull','push','check','price-points','app-store','testflight','completion','version','help'
  } elseif ($elements[1].Value -eq 'auth') {
    'login','check','logout'
  } elseif ($elements[1].Value -eq 'completion') {
    'bash','zsh','fish','powershell'
  } elseif ($elements[1].Value -eq 'app-store' -and $elements.Count -le 3) {
	'status','submit','release'
  } elseif ($elements[1].Value -eq 'testflight' -and $elements.Count -le 3) {
	'status','distribute'
  } elseif ($elements[1].Value -eq 'init') {
    '--bundle-id','--version','--platform','--locale','--config','--force'
  } elseif ($elements[1].Value -eq 'pull') {
    '--config','--dry-run','--allow-local-asset-deletions'
  } elseif ($elements[1].Value -eq 'push') {
    '--config','--dry-run','--allow-empty','--allow-irreversible','--allow-asset-deletions','--allow-availability-changes','--allow-commercial-changes'
  } elseif ($elements[1].Value -eq 'check') {
    '--config'
  } elseif ($elements[1].Value -eq 'price-points') {
    '--config','--territory'
  } elseif ($elements[1].Value -eq 'app-store') {
	'--config','--json','--build','--release-type','--earliest-release-date','--dry-run','--confirm'
  } elseif ($elements[1].Value -eq 'testflight') {
	'--config','--json','--group','--build','--dry-run','--confirm'
  } else { @() }
  $candidates | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
  }
}
`
