#!/bin/bash -eu

compile_go_fuzzer github.com/favonia/cloudflare-ddns/test/fuzzer ParseList fuzz_parse_list
compile_go_fuzzer github.com/favonia/cloudflare-ddns/test/fuzzer ParseExpression fuzz_parse_expression
