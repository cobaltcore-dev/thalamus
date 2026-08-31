Output ../public/operator-model-ready.gif

Set Width 2000
Set Height 1400
Set FontSize 22
Set TypingSpeed 10ms

Type `kubectl get models -n thalamus`
Enter

Sleep 3s

Type `curl -s https://api.thalamus.eu-de-2.cloud.sap/v1/models -H "Authorization: Bearer $THALAMUS_API_KEY" | jq .`
Enter

Sleep 5s

Type `curl -s https://api.thalamus.eu-de-2.cloud.sap/v1/chat/completions -H "Authorization: Bearer $THALAMUS_API_KEY" -H "Content-Type: application/json" -d '{"model":"Qwen/Qwen3.6-27B","messages":[{"role":"user","content":"What is Kubernetes in one sentence?"}]}' | jq '.choices[0].message.content'`
Enter

Sleep 20s
