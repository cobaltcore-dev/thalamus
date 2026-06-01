Output ../public/opencode.gif

Set Width 2000
Set Height 1200
Set FontSize 22
Set TypingSpeed 10ms

Type `cat ~/.config/opencode/opencode.json | jq '.provider.thalamus | del(.options.apiKey)'`
Enter

Sleep 5s

Type `opencode`
Enter

Sleep 2s

Type `What is the time complexity of Dijkstra's algorithm and why?`
Enter

Sleep 15s
