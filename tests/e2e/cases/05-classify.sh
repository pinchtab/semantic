#!/bin/bash
source /e2e/lib.sh

echo "  ── Classify: Error Classification ──"

# Element not found
result=$(semantic classify "could not find node with given id")
assert_contains "$result" "element_not_found" "classify: node not found"
assert_contains "$result" "recoverable: true" "classify: node not found is recoverable"

# Stale element
result=$(semantic classify "node is detached from the DOM")
assert_contains "$result" "element_stale" "classify: detached DOM"
assert_contains "$result" "recoverable: true" "classify: stale is recoverable"

# Not interactable
result=$(semantic classify "element is not visible on the page")
assert_contains "$result" "element_not_interactable" "classify: not visible"
assert_contains "$result" "recoverable: true" "classify: not interactable is recoverable"

# Navigation
result=$(semantic classify "frame was detached during navigation")
assert_contains "$result" "navigation" "classify: frame detached"
assert_contains "$result" "recoverable: true" "classify: navigation is recoverable"

# Network error
result=$(semantic classify "connection refused to localhost:9222")
assert_contains "$result" "network" "classify: connection refused"
assert_contains "$result" "recoverable: false" "classify: network is not recoverable"

# Unknown error
result=$(semantic classify "something completely random happened")
assert_contains "$result" "unknown" "classify: unknown error"
assert_contains "$result" "recoverable: false" "classify: unknown is not recoverable"

# Stdin mode
result=$(echo "timeout waiting for response" | semantic classify -)
assert_contains "$result" "network" "classify: stdin timeout"

summary "classify"
