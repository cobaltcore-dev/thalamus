Output ../public/operator-model-apply.gif

Set Width 2000
Set Height 1400
Set FontSize 22
Set TypingSpeed 10ms

Type `kubectl apply -f ../../examples/model-qwen3-6-27b-gpu.yaml`
Enter

Sleep 2s

Type `kubectl get models -n thalamus`
Enter

Sleep 5s

Type `kubectl get deploy,svc,sa,role,rolebinding,configmap,inferencepool,httproute -n thalamus | grep qwen | sed 's/[0-9]\{1,3\}\(\.[0-9]\{1,3\}\)\{3\}/REDACTED/g'`
Enter

Sleep 15s
