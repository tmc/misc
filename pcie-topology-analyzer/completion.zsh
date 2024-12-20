#compdef pcie-topology-analyzer

_pcie_topology_analyzer() {
    local -a opts
    opts=(
        '--count-tokens[Count tokens in output]'
        '--verbose[Enable verbose logging]'
        '--debug[Enable debug mode]'
        '--validate[Validate generated XML]'
        '--output-dir[Specify output directory]:directory:_files -/'
        '--help[Show help message]'
    )

    _arguments -s $opts
}

_pcie_topology_analyzer "$@"

