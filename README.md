# Ollama Openrouter Proxy

A proxy server that makes OpenRouter's free AI models available through an Ollama-compatible API, with intelligent retry logic and rate limit protection. I primarily use it for testing. 

## Features
- **Free Mode (Default)**: Automatically selects and uses free models from OpenRouter with intelligent fallback. Enabled by default unless `FREE_MODE=false` is set.
- **Model Filtering**: Create a `models-filter/filter` file with model name patterns (one per line). Supports partial matching - `gemini` matches `gemini-2.0-flash-exp:free`. Works in both free and non-free modes.
- **Tool Use Filtering**: Filter for only free models that support function calling/tool use by setting `TOOL_USE_ONLY=true`. Models are filtered based on their `supported_parameters` containing "tools" or "tool_choice".
- **Ollama-like API**: The server listens on `11434` and exposes endpoints similar to Ollama (e.g., `/api/chat`, `/api/tags`).
- **Model Listing**: Fetch a list of available models from OpenRouter.
- **Model Details**: Retrieve metadata about a specific model.
- **Streaming Chat**: Forward streaming responses from OpenRouter in a chunked JSON format that is compatible with Ollama’s expectations.

## Usage
You can provide your **OpenRouter** (OpenAI-compatible) API key through an environment variable:

### Environment Variable

    export OPENROUTER_API_KEY="your-openrouter-api-key"
    ./ollama-proxy

### Free Mode (Default Behavior)

The proxy operates in **free mode** by default, automatically selecting from available free models on OpenRouter. This provides cost-effective usage without requiring manual model selection.

    # Free mode is enabled by default - no configuration needed
    export OPENROUTER_API_KEY="your-openrouter-api-key"
    ./ollama-proxy

    # To disable free mode and use all available models
    export FREE_MODE=false
    export OPENROUTER_API_KEY="your-openrouter-api-key"
    ./ollama-proxy

    # To only use free models that support tool use/function calling
    export TOOL_USE_ONLY=true
    export OPENROUTER_API_KEY="your-openrouter-api-key"
    ./ollama-proxy

#### How Free Mode Works

- **Automatic Model Discovery**: Fetches and caches available free models from OpenRouter
- **Intelligent Fallback**: If a requested model fails, automatically tries other available free models
- **Failure Tracking**: Temporarily skips models that have recently failed (15-minute cooldown)
- **Model Prioritization**: Tries models in order of context length (largest first)
- **Cache Management**: Maintains a `free-models` file for quick startup and a `failures.db` SQLite database for failure tracking

Once running, the proxy listens on port `11434`. You can make requests to `http://localhost:11434` with your Ollama-compatible tooling.

## API Endpoints

The proxy provides both Ollama-compatible and OpenAI-compatible endpoints:

### Ollama API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/` | Health check - returns "Ollama is running" |
| `HEAD` | `/` | Health check (head request) |
| `GET` | `/api/tags` | List available models in Ollama format |
| `POST` | `/api/show` | Get model details |
| `POST` | `/api/chat` | Chat completion with streaming support |

#### Example Requests

**List Models:**
```bash
curl http://localhost:11434/api/tags
```

**Chat Completion:**
```bash
curl -X POST http://localhost:11434/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat-v3-0324:free",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "stream": true
  }'
```

**Model Details:**
```bash
curl -X POST http://localhost:11434/api/show \
  -H "Content-Type: application/json" \
  -d '{"name": "deepseek-chat-v3-0324:free"}'
```

### OpenAI API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/models` | List available models in OpenAI format |
| `POST` | `/v1/chat/completions` | Chat completion with streaming support |

#### Example Requests

**List Models (OpenAI format):**
```bash
curl http://localhost:11434/v1/models
```

**Chat Completion (OpenAI format):**
```bash
curl -X POST http://localhost:11434/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat-v3-0324:free",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "stream": false
  }'
```


## Docker Usage

### Using Docker Compose 

1. **Clone the repository and create environment file**:
   ```bash
   git clone https://github.com/your-username/ollama-openrouter-proxy.git
   cd ollama-openrouter-proxy
   cp .env.example .env
   ```

2. **Edit `.env` file with your OpenRouter API key**:
   ```bash
   OPENROUTER_API_KEY=your-openrouter-api-key
   FREE_MODE=true
   TOOL_USE_ONLY=false
   ```


3. **Optional: Create model filter**:
   ```bash
   mkdir -p models-filter
   echo "gemini" > models-filter/filter  # Only show Gemini models
   ```

4. **Optional: Enable tool use filtering**:
   Set `TOOL_USE_ONLY=true` in your `.env` file to only use models that support function calling/tool use. This filters models based on their `supported_parameters` containing "tools" or "tool_choice".

5. **Run with Docker Compose**:
   ```bash
   docker compose up -d
   ```

The service will be available at `http://localhost:11434`.

### Using Docker directly

```bash
docker build -t ollama-proxy .
docker run -p 11434:11434 -e OPENROUTER_API_KEY="your-openrouter-api-key" ollama-proxy

# To enable tool use filtering
docker run -p 11434:11434 -e OPENROUTER_API_KEY="your-openrouter-api-key" -e TOOL_USE_ONLY=true ollama-proxy
```


## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENROUTER_API_KEY` | Your OpenRouter API key (required) | - |
| `FREE_MODE` | Use only free models | `true` |
| `TOOL_USE_ONLY` | Filter for function-calling models only | `false` |
| `LOG_LEVEL` | Logging level (DEBUG, INFO, WARN, ERROR) | `INFO` |
| `PORT` | Server port | `11434` |
| `FAILURE_COOLDOWN_MINUTES` | Cooldown for temporary failures | `5` |
| `RATELIMIT_COOLDOWN_MINUTES` | Cooldown for rate limit errors | `1` |


## Acknowledgements
Inspiration for this project was [xsharov/enchanted-ollama-openrouter-proxy](https://github.com/xsharov/enchanted-ollama-openrouter-proxy) who took inspiration from [marknefedov](https://github.com/marknefedov/ollama-openrouter-proxy).
