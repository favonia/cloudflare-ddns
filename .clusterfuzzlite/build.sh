#!/bin/bash -eu

compile_go_fuzzer github.com/favonia/cloudflare-ddns/fuzz ParseList fuzz_parse_list
compile_go_fuzzer github.com/favonia/cloudflare-ddns/fuzz ParseExpression fuzz_parse_expression
