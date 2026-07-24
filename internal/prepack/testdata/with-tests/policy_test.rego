package main

import rego.v1

test_deny_pod_without_name if {
	result := deny with input as {"kind": "Pod"}
	count(result) > 0
}

test_allow_pod_with_name if {
	result := deny with input as {"kind": "Pod", "metadata": {"name": "my-pod"}}
	count(result) == 0
}
