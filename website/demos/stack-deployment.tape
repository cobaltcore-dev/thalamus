Output ../public/stack-deployment.gif

Set Width 2200
Set Height 1000
Set FontSize 22
Set TypingSpeed 10ms

Type `for ns in thalamus-system thalamus; do helm list -n $ns; done`
Enter

Sleep 5s

Type `kubectl get pods -n thalamus-system | grep -v nvidia`
Enter

Sleep 5s

Type `kubectl get pods -n thalamus`
Enter

Sleep 5s
