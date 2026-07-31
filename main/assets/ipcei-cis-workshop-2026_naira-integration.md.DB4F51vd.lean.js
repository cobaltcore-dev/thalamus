import{Z as a,Q as d,j as r,m as t,o as s,a9 as m,g as n,n as p}from"./chunks/framework.B5EQ7QKW.js";const g=[{id:"c6406518-ad90-4de7-a770-5b479061ac98",user_id:"14d3e647-74b9-4cc1-8db4-0c87d028cb57",title:"🧠 Thalamus Available Models",chat:{id:"c6406518-ad90-4de7-a770-5b479061ac98",title:"🧠 Thalamus Available Models",models:["Qwen/Qwen3.6-27B"],history:{currentId:"aad4ac42-112e-44e8-bc38-90692cb743a8",messages:{"3cc28e1e-c968-442e-89ff-7c06a0289d71":{id:"3cc28e1e-c968-442e-89ff-7c06a0289d71",parentId:null,childrenIds:["d88c67fb-154d-4c94-b463-581d7091491b"],role:"user",content:"Which models are available in thalamus?",timestamp:1785479739,models:["Qwen/Qwen3.6-27B"]},"d88c67fb-154d-4c94-b463-581d7091491b":{id:"d88c67fb-154d-4c94-b463-581d7091491b",parentId:"3cc28e1e-c968-442e-89ff-7c06a0289d71",childrenIds:["0daa8010-a3f6-47ad-b4fe-f8d566dd9357"],role:"assistant",content:"",done:!0,model:"Qwen/Qwen3.6-27B",timestamp:1785479739,output:[{type:"reasoning",id:"r_3e3d38c080234738a2a1f76e",status:"completed",start_tag:"<think>",end_tag:"</think>",attributes:{type:"reasoning_content"},content:[{type:"output_text",text:' The user is asking which models are available in "thalamus". This sounds like it could be a data catalog or model registry. I should use the catalog tools to explore nodes, specifically filtering for models. Let me start by listing node kinds to understand the catalog structure, then list nodes of kind "model" if that exists.'}],summary:null,started_at:17854797398587627e-7,ended_at:17854797402774315e-7,duration:0},{type:"function_call",id:"functions.listKinds:0",call_id:"functions.listKinds:0",name:"listKinds",arguments:"{}",status:"completed"},{type:"function_call_output",id:"fco_229a442e1df54593b3c90caf",call_id:"functions.listKinds:0",output:[{type:"input_text",text:`{
  "results": [
    "application",
    "dataset",
    "model",
    "deployment",
    "service",
    "Kustomization.fluxcd",
    "HelmChart.fluxcd",
    "git_repository",
    "trained_on",
    "uses_model",
    "calls",
    "sourced_from",
    "describes",
    "deployed_from",
    "derived_from"
  ]
}`}],status:"completed"},{type:"function_call",id:"functions.listNodes:1",call_id:"functions.listNodes:1",name:"listNodes",arguments:'{"filter": "kind=\\"model\\""}',status:"completed"},{type:"function_call_output",id:"fco_a18412a0e60e4be1b6e8c4c3",call_id:"functions.listNodes:1",output:[{type:"input_text",text:`{
  "nodes": [
    {
      "name": "nodes/model/litellm/idp-claude-sonnet",
      "kind": "model",
      "path": "litellm/idp-claude-sonnet",
      "pluginClaims": [
        {
          "plugin": "litellm",
          "props": {
            "owned_by": "openai"
          }
        }
      ]
    },
    {
      "name": "nodes/model/litellm/idp-embedding-small",
      "kind": "model",
      "path": "litellm/idp-embedding-small",
      "pluginClaims": [
        {
          "plugin": "litellm",
          "props": {
            "owned_by": "openai"
          }
        }
      ]
    },
    {
      "name": "nodes/model/litellm/idp-gemini-flash",
      "kind": "model",
      "path": "litellm/idp-gemini-flash",
      "pluginClaims": [
        {
          "plugin": "litellm",
          "props": {
            "owned_by": "openai"
          }
        }
      ]
    },
    {
      "name": "nodes/model/litellm/idp-gpt-4o-mini",
      "kind": "model",
      "path": "litellm/idp-gpt-4o-mini",
      "pluginClaims": [
        {
          "plugin": "litellm",
          "props": {
            "owned_by": "openai"
          }
        }
      ]
    },
    {
      "name": "nodes/model/mlflow/fraud-detector",
      "kind": "model",
      "path": "mlflow/fraud-detector",
      "pluginClaims": [
        {
          "plugin": "mlflow",
          "props": {
            "description": ""
          }
        }
      ]
    },
    {
      "name": "nodes/model/thalamus/Qwen/Qwen-AgentWorld-35B-A3B",
      "kind": "model",
      "path": "thalamus/Qwen/Qwen-AgentWorld-35B-A3B",
      "pluginClaims": [
        {
          "plugin": "thalamus",
          "props": {
            "engine_args": "[\\"--max-model-len=32768\\",\\"--tensor-parallel-size=2\\"]",
            "engine_image": "vllm/vllm-openai:v0.9.0",
            "engine_type": "",
            "epp_type": "",
            "has_epp": "false",
            "name": "qwen-agentworld-35b-a3b",
            "namespace": "demo-inference",
            "node_selector": "{\\"example.com/accelerator\\":\\"example-gpu-80gb\\"}",
            "phase": "",
            "weights_hf_repo_id": "Qwen/Qwen-AgentWorld-35B-A3B",
            "weights_type": "hf"
          }
        }
      ]
    },
    {
      "name": "nodes/model/thalamus/Qwen/Qwen3.6-27B",
      "kind": "model",
      "path": "thalamus/Qwen/Qwen3.6-27B",
      "pluginClaims": [
        {
          "plugin": "thalamus",
          "props": {
            "engine_args": "[\\"--max-model-len=32768\\",\\"--tensor-parallel-size=2\\"]",
            "engine_image": "vllm/vllm-openai:v0.9.0",
            "engine_type": "",
            "epp_type": "",
            "has_epp": "false",
            "name": "qwen3-6-27b",
            "namespace": "demo-inference",
            "node_selector": "{\\"example.com/accelerator\\":\\"example-gpu-80gb\\"}",
            "phase": "",
            "weights_hf_repo_id": "Qwen/Qwen3.6-27B",
            "weights_type": "hf"
          }
        }
      ]
    },
    {
      "name": "nodes/model/thalamus/google/gemma-4-26B-A4B-it",
      "kind": "model",
      "path": "thalamus/google/gemma-4-26B-A4B-it",
      "pluginClaims": [
        {
          "plugin": "thalamus",
          "props": {
            "engine_args": "[\\"--max-model-len=8192\\",\\"--tensor-parallel-size=2\\"]",
            "engine_image": "vllm/vllm-openai:v0.9.0",
            "engine_type": "",
            "epp_image": "registry.example.com/llm-d/epp:v0.3.0",
            "epp_type": "",
            "has_epp": "true",
            "name": "gemma-4-26b-a4b-it",
            "namespace": "demo-inference",
            "node_selector": "{\\"example.com/accelerator\\":\\"example-gpu-80gb\\"}",
            "phase": "",
            "weights_hf_repo_id": "google/gemma-4-26B-A4B-it",
            "weights_type": "hf"
          }
        }
      ]
    },
    {
      "name": "nodes/model/thalamus/google/gemma-4-31B-it",
      "kind": "model",
      "path": "thalamus/google/gemma-4-31B-it",
      "pluginClaims": [
        {
          "plugin": "thalamus",
          "props": {
            "engine_args": "[\\"--max-model-len=8192\\",\\"--tensor-parallel-size=2\\"]",
            "engine_image": "vllm/vllm-openai:v0.9.0",
            "engine_type": "",
            "epp_type": "",
            "has_epp": "false",
            "name": "gemma-4-31b-it",
            "namespace": "demo-inference",
            "node_selector": "{\\"example.com/accelerator\\":\\"example-gpu-80gb\\"}",
            "phase": "",
            "weights_hf_repo_id": "google/gemma-4-31B-it",
            "weights_type": "hf"
          }
        }
      ]
    },
    {
      "name": "nodes/model/thalamus/jinaai/jina-embeddings-v5-text-small-retrieval",
      "kind": "model",
      "path": "thalamus/jinaai/jina-embeddings-v5-text-small-retrieval",
      "pluginClaims": [
        {
          "plugin": "thalamus",
          "props": {
            "engine_args": "[\\"--max-model-len=8192\\",\\"--task=embed\\"]",
            "engine_image": "vllm/vllm-openai:v0.9.0",
            "engine_type": "",
            "epp_type": "",
            "has_epp": "false",
            "name": "jina-embeddings-v5-text-small",
            "namespace": "demo-inference",
            "node_selector": "{\\"example.com/accelerator\\":\\"example-gpu-24gb\\"}",
            "phase": "",
            "weights_hf_repo_id": "jinaai/jina-embeddings-v5-text-small-retrieval",
            "weights_type": "hf"
          }
        }
      ]
    },
    {
      "name": "nodes/model/thalamus/openai/gpt-oss-120b",
      "kind": "model",
      "path": "thalamus/openai/gpt-oss-120b",
      "pluginClaims": [
        {
          "plugin": "thalamus",
          "props": {
            "engine_args": "[\\"--max-model-len=32768\\",\\"--tensor-parallel-size=8\\"]",
            "engine_image": "vllm/vllm-openai:v0.9.0",
            "engine_type": "",
            "epp_image": "registry.example.com/llm-d/epp:v0.3.0",
            "epp_type": "",
            "has_epp": "true",
            "name": "gpt-oss-120b",
            "namespace": "demo-inference",
            "node_selector": "{\\"example.com/accelerator\\":\\"example-gpu-80gb\\"}",
            "phase": "",
            "weights_hf_repo_id": "openai/gpt-oss-120b",
            "weights_type": "hf"
          }
        }
      ]
    }
  ],
  "totalSize": 13
}`}],status:"completed"},{type:"reasoning",id:"r_dfc9f038d3074c29bce896b1",status:"completed",start_tag:"<think>",end_tag:"</think>",attributes:{type:"reasoning_content"},content:[{type:"output_text",text:` The user asked which models are available in thalamus. I have the list. I should present it clearly. The models in thalamus are those with paths starting with "thalamus/". There are also litellm and mlflow models, but they are not thalamus-specific unless the user is asking broadly about the catalog. They specifically said "in thalamus" so I should focus on thalamus models.

Let me list them:
1. thalamus/Qwen/Qwen-AgentWorld-35B-A3B
2. thalamus/Qwen/Qwen3.6-27B
3. thalamus/google/gemma-4-26B-A4B-it
4. thalamus/google/gemma-4-31B-it
5. thalamus/jinaai/jina-embeddings-v5-text-small-retrieval
8. thalamus/openai/gpt-oss-120b

I should also note there are litellm proxy models available (idp-claude-sonnet, idp-embedding-small, idp-gemini-flash, idp-gpt-4o-mini) and an mlflow model (fraud-detector), but they are not in the thalamus namespace. But maybe the user is asking about the catalog named thalamus. Since they said "in thalamus", I'll focus on those. I can also mention the others briefly as additional context.

Actually, thalamus here refers to the catalog system. Let me just answer with the thalamus models.`}],summary:null,started_at:17854797408014464e-7,ended_at:1785479742279684e-6,duration:1},{type:"message",id:"msg_46ad54d61d6d45d68614368f",status:"completed",role:"assistant",content:[{type:"output_text",text:"There are **6 models** registered in the `thalamus` namespace:\n\n| Path / Name | Weights source | Notes |\n|---|---|---|\n| `thalamus/Qwen/Qwen-AgentWorld-35B-A3B` | Hugging Face `Qwen/Qwen-AgentWorld-35B-A3B` | vLLM, 2x example-gpu-80gb, max 32k |\n| `thalamus/Qwen/Qwen3.6-27B` | Hugging Face `Qwen/Qwen3.6-27B` | vLLM, 2x example-gpu-80gb, max 32k |\n| `thalamus/google/gemma-4-26B-A4B-it` | Hugging Face `google/gemma-4-26B-A4B-it` | vLLM, 2x example-gpu-80gb, max 8k, has EPP |\n| `thalamus/google/gemma-4-31B-it` | Hugging Face `google/gemma-4-31B-it` | vLLM, 2x example-gpu-80gb, max 8k |\n| `thalamus/jinaai/jina-embeddings-v5-text-small-retrieval` | Hugging Face `jinaai/jina-embeddings-v5-text-small-retrieval` | vLLM embed, 1x example-gpu-24gb, max 8k |\n| `thalamus/openai/gpt-oss-120b` | Hugging Face `openai/gpt-oss-120b` | vLLM, 8x example-gpu-80gb, max 32k, has EPP |\n\nThe catalog also has non-`thalamus` models if relevant:\n\n- **litellm proxy models:** `idp-claude-sonnet`, `idp-embedding-small`, `idp-gemini-flash`, `idp-gpt-4o-mini`\n- **mlflow:** `fraud-detector`"}]}]},"0daa8010-a3f6-47ad-b4fe-f8d566dd9357":{id:"0daa8010-a3f6-47ad-b4fe-f8d566dd9357",parentId:"d88c67fb-154d-4c94-b463-581d7091491b",childrenIds:["aad4ac42-112e-44e8-bc38-90692cb743a8"],role:"user",content:"What inference engine and settings does Qwen3.6-27B use?",timestamp:1785479828,models:["Qwen/Qwen3.6-27B"]},"aad4ac42-112e-44e8-bc38-90692cb743a8":{id:"aad4ac42-112e-44e8-bc38-90692cb743a8",parentId:"0daa8010-a3f6-47ad-b4fe-f8d566dd9357",childrenIds:[],role:"assistant",content:"",done:!0,model:"Qwen/Qwen3.6-27B",timestamp:1785479828,output:[{type:"reasoning",id:"r_b7d326d8f1454c8993f1da0f",status:"completed",start_tag:"<think>",end_tag:"</think>",attributes:{type:"reasoning_content"},content:[{type:"output_text",text:" The user is asking about a specific model we already have full details for in the previous listNodes response. Let me provide the precise information."}],summary:null,started_at:17854798283314655e-7,ended_at:17854798285420763e-7,duration:0},{type:"message",id:"msg_0f4e99f1d3db4deaad7d2d91",status:"completed",role:"assistant",content:[{type:"output_text",text:"`thalamus/Qwen/Qwen3.6-27B` uses the following inference engine and settings:\n\n- **Engine image:** `vllm/vllm-openai:v0.9.0`\n- **Engine type:** vLLM OpenAI-compatible server\n- **Model weights:** Hugging Face `Qwen/Qwen3.6-27B`\n- **Engine arguments:**\n  - `--max-model-len=32768`\n  - `--tensor-parallel-size=2`\n- **GPU target:** 2x Example GPU 80GB (`example-gpu-80gb`)\n- **Namespace:** `demo-inference`\n- **EPP enabled:** No (`has_epp: false`)"}]}]}}},messages:[{role:"user",content:"Which models are available in thalamus?"}],files:[],tags:[],timestamp:1785479739653},updated_at:1785479831,created_at:1785479739,share_id:null,archived:!1,pinned:!1,meta:{tags:["general"]},folder_id:null,tasks:null,summary:null}],_=JSON.parse('{"title":"Naira Integration","description":"","frontmatter":{"title":"Naira Integration"},"headers":[],"relativePath":"ipcei-cis-workshop-2026/naira-integration.md","filePath":"ipcei-cis-workshop-2026/naira-integration.md","lastUpdated":1785510927000}'),c={name:"ipcei-cis-workshop-2026/naira-integration.md"},b=Object.assign(c,{setup(h){const l=g[0].chat.history.messages;return(u,e)=>{const i=a("ImageCarousel"),o=a("ChatTranscript");return d(),r("div",null,[e[0]||(e[0]=t("",7)),s(i,{images:[{src:"/hackathon-2026/naira-chat-engine.png",alt:"Engine settings response"},{src:"/hackathon-2026/naira-chat-models.png",alt:"Model list response"},{src:"/hackathon-2026/naira-chat-tool-call.png",alt:"Tool call detail"}],caption:"Open WebUI chat using the Naira MCP server to answer live questions about Thalamus-managed models."}),e[1]||(e[1]=t("",5)),s(o,{messages:m(l)},null,8,["messages"]),e[2]||(e[2]=n("h2",{id:"what-s-next",tabindex:"-1"},[p("What's Next "),n("a",{class:"header-anchor",href:"#what-s-next","aria-label":`Permalink to "What's Next"`},"​")],-1)),e[3]||(e[3]=n("p",null,"The plugin is a working draft. The next steps are to harden it for production use: define a stable schema for the Thalamus model node, add error handling and resync logic, and publish the plugin so it can be installed alongside other Naira plugins.",-1))])}}});export{_ as __pageData,b as default};
