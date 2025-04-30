# 🧠 Multiple LLM Question Generator API

This Go project provides an API to upload PDF files and generate multiple-choice questions using different LLM providers like TogetherAI, Cohere, Gemini, and more.

---

## 🚀 Features

- Upload PDF files and extract text  
- Choose from multiple LLM models  
- Generate customized MCQs with answers  
- Store questions, prompts, and feedback in a SQLite database

---

## 📦 Installation

1. **Clone the repository:**

```bash
   git clone https://github.com/21f3000703/chat-with-pdf.git 
   cd chat-with-pdf
```

2. **Initialize Go modules and download dependencies:**

```bash
   go mod tidy
```

---

## ▶️ Running the Project

To start the server, update the api keys in `multiplellm.go` file and run:

```bash
   go run multiplellm.go
```

The server will run on http://localhost:8089 by default.

---

## 🧪 Testing the API

You can test the `/upload` endpoint using the following `curl` command:


```bash
   curl -X POST "http://localhost:8089/upload" \
     -H "Content-Type: multipart/form-data" \
     -F "file=@/full/path/to/Academy_Awards.pdf" \
     -F "num_questions=1" \
     -F "llm_model=togetherai"
```

### 📌 Parameters

- `file`: The PDF file you want to upload  
- `num_questions`: Number of questions to generate  
- `llm_model`: One of `togetherai`, `cohere`, `gemini`, etc.

---
