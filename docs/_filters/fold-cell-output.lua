-- Fold executed cell output into its code block as trailing comments.
-- Keeps output attached to its cell in rendered logs. No-op without
-- outputs (e.g. --no-execute renders) and for non-bash cells.
function Div(div)
  if not div.classes:includes("cell") then
    return nil
  end
  local code = nil
  local out = {}
  local keep = {}
  local droppable = true
  for _, b in ipairs(div.content) do
    if b.t == "CodeBlock" and b.classes:includes("cell-code") and not code then
      code = b
      table.insert(keep, b)
    elseif b.t == "Div" and b.classes:includes("cell-output") then
      for _, ob in ipairs(b.content) do
        if ob.t == "CodeBlock" then
          table.insert(out, ob.text)
        else
          droppable = false
        end
      end
      if not droppable then
        table.insert(keep, b)
      end
    else
      table.insert(keep, b)
    end
  end
  if code and #out > 0 and code.classes:includes("bash") then
    local lines = { code.text }
    for _, t in ipairs(out) do
      for line in (t .. "\n"):gmatch("([^\n]*)\n") do
        if line == "" then
          table.insert(lines, "#")
        else
          table.insert(lines, "# " .. line)
        end
      end
    end
    code.text = table.concat(lines, "\n")
    div.content = keep
  end
  return div
end
