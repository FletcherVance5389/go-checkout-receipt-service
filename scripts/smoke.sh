#!/bin/sh
set -eu

curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data '{"order_id":"A-42","customer_email":"chenhua@changba.com","status":"paid","total_cents":2599,"currency":"USD"}' \
  http://localhost:8080/order-updates
