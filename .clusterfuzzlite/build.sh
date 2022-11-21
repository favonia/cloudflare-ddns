#!/bin/bash -eu

compile_go_fuzzer github.com/favonia/cloudflare-ddns/fuzzer FuzzParseList fuzz_parse_list
compile_go_fuzzer github.com/favonia/cloudflare-ddns/fuzzer FuzzParseExpression fuzz_parse_expression
