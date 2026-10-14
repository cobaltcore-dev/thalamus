-- Display-only substitution of safe env vars in code blocks (pandoc stage).
-- Executed cells use live $VAR values; this rewrites display to copy-pasteable literals.
-- Never list secrets here (e.g. HF_TOKEN stays as $HF_TOKEN).
local allowlist = {
  "REF",
  "ROOT",
  "REPOSITORY",
  "THALAMUS_CHART_LOCATION",
  "THALAMUS_CHART_VERSION",
  "THALAMUS_IMAGE_TAG",
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
