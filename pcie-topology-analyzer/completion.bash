# Bash completion for pcie-topology-analyzer
_pcie_topology_analyzer() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="--count-tokens --verbose --debug --validate --output-dir --help"

    case "${prev}" in
        --output-dir)
            COMPREPLY=( $(compgen -d -- "${cur}") )
            return 0
            ;;
        *)
            ;;
    esac

    COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _pcie_topology_analyzer pcie-topology-analyzer

