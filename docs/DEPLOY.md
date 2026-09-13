# TLS va sessiya arxivi

## Dev sertifikatlari

```sh
openssl ecparam -name prime256v1 -genkey -noout -out ca.key
openssl req -x509 -new -nodes -key ca.key -sha256 -days 365 -subj '/CN=alora-dev-ca' -out ca.crt
openssl ecparam -name prime256v1 -genkey -noout -out server.key
openssl req -new -key server.key -subj '/CN=localhost' -out server.csr
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -sha256 -out server.crt
openssl ecparam -name prime256v1 -genkey -noout -out client.key
openssl req -new -key client.key -subj '/CN=alora-client' -out client.csr
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -sha256 -out client.crt
```

TLS yoqilganda konfiguratsiyadagi qiymatlar sertifikat mazmuni emas, fayl yo'llarini
ko'rsatuvchi ENV nomlaridir. Server uchun `ALORA_GRPC_TLS_CERT_FILE`,
`ALORA_GRPC_TLS_KEY_FILE`, `ALORA_GRPC_TLS_CA_FILE`; Django klienti uchun
`ALORA_BACKEND_TLS_CERT_FILE`, `ALORA_BACKEND_TLS_KEY_FILE`, `ALORA_BACKEND_TLS_CA_FILE`.
Ikkala yo'nalishda ham TLS 1.3 va klient sertifikatini tekshirish talab qilinadi.

Django gRPC serveri `grpc.ssl_server_credentials` bilan server private key/certificate
juftligini va yuqoridagi CA (`root_certificates`) ni qabul qilishi, `require_client_auth=True`
ni yoqishi kerak. CA bir xil bo'lmasa handshake rad etiladi.
