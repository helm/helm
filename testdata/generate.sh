#!/bin/sh

openssl req -new -config openssl.conf -key key.pem -out key.csr
openssl ca -config openssl.conf -create_serial -batch -in key.csr -out crt.pem -key rootca.key -cert rootca.crt
