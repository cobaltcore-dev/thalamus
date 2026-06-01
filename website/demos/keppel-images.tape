Output ../public/keppel-images.gif

Set Width 2000
Set Height 620
Set FontSize 22
Set TypingSpeed 10ms

Type `for ns in thalamus-system thalamus; do kubectl get pods -n $ns -o jsonpath="{range .items[*]}{range .spec.containers[*]}{.image}{\"\n\"}{end}{end}"; done | sort | uniq`
Enter

Sleep 10s
