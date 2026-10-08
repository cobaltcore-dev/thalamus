-- Quarto Lua filter: display-only substitution of safe env vars in code blocks.
-- Runs at the pandoc stage (post-execution): executed cells always use live
-- $VAR values from the environment; this only rewrites what readers see, so
-- rendered docs show copy-pasteable literals (e.g. 0.0.0-main on main).
-- Secrets must NEVER be listed here (e.g. HF_TOKEN keeps showing $HF_TOKEN,
-- which is correct: readers supply their own token).
local allowlist = {
  "REF",
  "THALAMUS_CHART_LOCATION",
  "THALAMUS_CHART_VERSION",
  "OPERATOR_IMAGE_TAG",
}

function CodeBlock(block)
  local text = block.text
  for _, name in ipairs(allowlist) do
    local val = os.getenv(name)
    if val ~= nil then
      text = text:gsub("%${" .. name .. "}", function()
        return val
      end)
      text = text:gsub("%$" .. name .. "%f[^%w_]", function()
        return val
      end)
    end
  end
  -- Omit flags left empty: identical semantics to executing with "".
  text = text:gsub('%s+%-%-set%s+%S-=""', "")
  text = text:gsub('%s+%-%-version%s+""', "")
  block.text = text
  return block
end
