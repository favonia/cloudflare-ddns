#!/bin/bash -eu

go install github.com/AdamKorcz/go-118-fuzz-build@v0.0.0-20221121202950-b2031950a318
go get github.com/AdamKorcz/go-118-fuzz-build/testing@v0.0.0-20221121202950-b2031950a318

compile_native_go_fuzzer github.com/favonia/cloudflare-ddns/test/fuzzer FuzzParseList fuzz_parse_list
compile_native_go_fuzzer github.com/favonia/cloudflare-ddns/test/fuzzer FuzzParseExpression fuzz_parse_expression
