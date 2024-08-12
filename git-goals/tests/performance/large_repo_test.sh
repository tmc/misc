#!/bin/bash

# Create a large repository with many goals
setup_large_repo() {
    mkdir large_repo
    cd large_repo
    git init
    for i in {1..1000}; do
        git goals create "Goal "
    done
}

# Test performance of list command
test_list_performance() {
    time git goals list
}

# Test performance of report command
test_report_performance() {
    time git goals report
}

# Run tests
setup_large_repo
test_list_performance
test_report_performance

# TODO: Implement more comprehensive performance tests
