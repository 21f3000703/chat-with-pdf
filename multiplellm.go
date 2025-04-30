package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"errors"
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var API_KEYS = map[string]string{
    "deepinfra": "your-deepinfra-api",
    "gemini":    "your-gemini-api",
    "cohere":    "your-cohere-api",
    "quizapi":   "your-quizapi-api",
    "togetherai": "your-togetherai-api",
}

var LLM_APIS = map[string]string{
	"deepinfra": "https://api.deepinfra.com/v1/openai/chat/completions",
	"gemini":    "https://generativelanguage.googleapis.com/v1/models/gemini-1.5-pro-002:generateContent?key=your-gemini-api",
	"cohere":    "https://api.cohere.com/v1/generate",
	"quizapi":   "https://openai37.p.rapidapi.com/chat-completion",
	"togetherai":"https://api.together.xyz/v1/chat/completions",
}

type Question struct {
	ID            int      `json:"id"`
	Prompt        string   `json:"prompt"`
	QuestionText  string   `json:"question_text"`
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
	LLMUsed       string   `json:"llm_used"`
	Feedback      int      `json:"feedback"` // 1 for like, -1 for dislike
	Context       string   `json:"context"`   // Add this field if needed
}


func main() {
	// Initialize database and schema
	db, err := sql.Open("sqlite3", "./prompts.db")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to database successfully.") // Debug print
	defer db.Close()

	// Create the prompts table if it doesn't exist
	// Create the prompts table if it doesn't exist
    createTableSQL := `
    CREATE TABLE IF NOT EXISTS prompts (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    prompt TEXT,
	    context TEXT,
	    question TEXT,
	    options TEXT,
	    correct_answer TEXT,
	    llm_used TEXT,
        FEEDBACK INTEGER DEFAULT 0
    );
    `
    _, err = db.Exec(createTableSQL)
    if err != nil {
	    log.Fatal("Error creating table:", err)
    }else {
		fmt.Println("Table created or already exists.") // Debug
	}


	// Setup HTTP handlers
	http.HandleFunc("/", welcomeHandler)
	http.HandleFunc("/select-llm", func(w http.ResponseWriter, r *http.Request) {
		handleLLMSelection(w, r, db)
	})
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		handleUpload(w, r, db)
	})
	http.HandleFunc("/feedback", func(w http.ResponseWriter, r *http.Request) {
		handleFeedback(w, r, db)
	})
	http.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		handleHistory(w, r, db)
	})

	log.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"message": "Welcome to the LLM Question Generator API!"}`)
}

// Handle LLM selection
func handleLLMSelection(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// List available LLM models
	llms := []string{"deepinfra", "gemini", "cohere", "quizapi","togetherai"}

	// Send response with LLM options
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Select an LLM to use",
		"llms":    llms,
	})
}

// Handle file upload and question generation
func handleUpload(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Parse the form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get the file and the selected LLM model
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File upload failed", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get the LLM model selection and the number of questions
	selectedModel := r.FormValue("llm_model")
	if selectedModel == "" {
		http.Error(w, "LLM model selection is required", http.StatusBadRequest)
		return
	}

	numQuestionsStr := r.FormValue("num_questions")
	numQuestions, err := strconv.Atoi(numQuestionsStr)
	if err != nil || numQuestions <= 0 {
		http.Error(w, "Invalid number of questions", http.StatusBadRequest)
		return
	}

	// Extract text from the PDF
	text, err := extractTextFromPDF(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate questions using the selected LLM
	questions, err := generateQuestionsWithLLM(text, numQuestions, selectedModel, db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Questions generated successfully",
		"questions": questions,
	})
}

// Extract text from PDF file (Placeholder)
func extractTextFromPDF(file io.Reader) (string, error) {
	// Here you can use any PDF parsing library (such as pdfplumber or another Go package) to extract text.
	return "Sample text extracted from the PDF", nil
}

// Generate questions using selected LLM
func generateQuestionsWithLLM(text string, numQuestions int, selectedModel string, db *sql.DB) ([]Question, error) {
	var questions []Question
	apiURL, exists := LLM_APIS[selectedModel]
	if !exists {
		return nil, errors.New("invalid model selected")
	}

	// Prepare prompt
	prompt := fmt.Sprintf("Generate %d multiple-choice questions based on the following text:\n\n%s", numQuestions, text)
	fmt.Println("Prompt:", prompt)

	// Prepare request body
	var requestBody []byte
	var err error

	switch selectedModel {
	case "deepinfra":
		requestBody, err = json.Marshal(map[string]interface{}{
			"model": "mistralai/Mistral-7B-Instruct-v0.1",
			"messages": []map[string]string{
				{"role": "system", "content": "You are an AI that generates multiple-choice questions."},
				{"role": "user", "content": prompt},
			},
			"max_tokens": 200,
			"temperature": 0.7,
			"top_p":      1,
		})

	case "gemini":
		requestBody, err = json.Marshal(map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{
					{"text": prompt},
				}},
			},
		})

	case "cohere":
		requestBody, err = json.Marshal(map[string]interface{}{
			"model":      "command-r-plus",
			"prompt":     prompt,
			"max_tokens": 200,
			"temperature": 0.7,
		})

	case "quizapi":
		requestBody, err = json.Marshal(map[string]interface{}{
			"model":  "gpt-3.5-turbo",
			"prompt": prompt,
		})
		if err != nil {
			log.Fatalf("Error marshalling request body: %v", err)
		}
		
    case "togetherai":
		requestBody, err = json.Marshal(map[string]interface{}{
			"model": "mistralai/Mistral-7B-Instruct-v0.2",
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"max tokens":512,
			"temperature": 0.7,
		})

	default:
		return nil, fmt.Errorf("invalid model selected")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
    // Ensure requestBody is used
	if requestBody == nil {
		log.Fatalf("Error: requestBody is nil")
	}

	
	// Set headers
	headers := map[string]string{
		"Content-Type":    "application/json",
		"X-RapidAPI-Key":  API_KEYS[selectedModel],
		"X-RapidAPI-Host": "openai37.p.rapidapi.com",
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}


	// Add headers
	if selectedModel != "gemini" {
		req.Header.Set("Authorization", "Bearer "+API_KEYS[selectedModel])
	}
	req.Header.Set("Content-Type", "application/json")

	if selectedModel != "deepinfra"{
		req.Header.Set("Authorization", "Bearer "+API_KEYS[selectedModel])
	}
	req.Header.Set("Content-Type", "application/json")

	if selectedModel != "quizapi"{
		req.Header.Set("Content-Type", "application/json")
	    req.Header.Set("X-RapidAPI-Key", API_KEYS[selectedModel])
	    req.Header.Set("X-RapidAPI-Host", "openai37.p.rapidapi.com")
	}
	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling API: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	log.Printf("API Response: %s", string(body))

	// Parse JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}
    if selectedModel != "deepinfra"{
		req.Header.Set("Authorization", "Bearer "+API_KEYS[selectedModel])
	}
	req.Header.Set("Content-Type", "application/json")



	// Extract questions based on model response format
	switch selectedModel {
	case "cohere":
		if generations, ok := response["generations"].([]interface{}); ok && len(generations) > 0 {
			for _, generation := range generations {
				if genMap, ok := generation.(map[string]interface{}); ok {
					if generatedText, exists := genMap["text"].(string); exists {
						options := []string{"Option A", "Option B", "Option C", "Option D"}
						questions = append(questions, Question{
							QuestionText:  generatedText,
							Options:       options,
							CorrectAnswer: "Option A",
							LLMUsed:       selectedModel,
						})
						storeQuestion(db, prompt, text, generatedText, options, "Option A", selectedModel,0)
					}
				}
			}
		}

	case "gemini":
		if candidates, ok := response["candidates"].([]interface{}); ok && len(candidates) > 0 {
			for _, candidate := range candidates {
				if candidateMap, ok := candidate.(map[string]interface{}); ok {
					if content, exists := candidateMap["content"].(map[string]interface{}); exists {
						if parts, partExists := content["parts"].([]interface{}); partExists && len(parts) > 0 {
							if textMap, textExists := parts[0].(map[string]interface{}); textExists {
								if generatedText, ok := textMap["text"].(string); ok {
									options := []string{"Option A", "Option B", "Option C", "Option D"}
									questions = append(questions, Question{
										QuestionText:  generatedText,
										Options:       options,
										CorrectAnswer: "Option A",
										LLMUsed:       selectedModel,
									})
									storeQuestion(db, prompt, text, generatedText, options, "Option A", selectedModel,0)
								}
							}
						}
					}
				}
			}
		}
	case "togetherai":
		if choices, ok := response["choices"].([]interface{}); ok && len(choices) > 0 {
			for _, choice := range choices {
				if choiceMap, ok := choice.(map[string]interface{}); ok {
					if message, exists := choiceMap["message"].(map[string]interface{}); exists {
						if generatedText, textExists := message["content"].(string); textExists {
							options := []string{"Option A", "Option B", "Option C", "Option D"}
							questions = append(questions, Question{
								QuestionText:  generatedText,
								Options:       options,
								CorrectAnswer: "Option A",
								LLMUsed:       selectedModel,
						    })
						    storeQuestion(db, prompt, text, generatedText, options, "Option A", selectedModel,0)
						}
					}
				}
			}
		}
	
	
    case "deepinfra":
        if choices, ok := response["choices"].([]interface{}); ok && len(choices) > 0 {
            if choiceMap, ok := choices[0].(map[string]interface{}); ok {  
                if message, exists := choiceMap["message"].(map[string]interface{}); exists { 
                    if content, exists := message["content"].(string); exists { 
                        questions = append(questions, Question{
                            QuestionText: content,
                            Options: []string{"Option A", "Option B", "Option C", "Option D"},
                            CorrectAnswer: "Option A",
                            LLMUsed: selectedModel,
                        })
                        db.Exec("INSERT INTO prompts (prompt, response) VALUES (?, ?)", prompt, content)
                    }
                }
            }
        }
    }
	return questions, nil
    
}
// Store question in the database
func storeQuestion(db *sql.DB, prompt, context, question string, options []string, correctAnswer, llmUsed string, feedback int) {
	// Convert options to a string if necessary (e.g., join the options as a comma-separated list)
	optionsStr := strings.Join(options, ", ")

	// Now, insert the question into the database
	_, err := db.Exec("INSERT INTO prompts(prompt, context, question, options, correct_answer, llm_used, feedback) VALUES(?, ?, ?, ?, ?, ?, ?)", 
    prompt, context, question, optionsStr, correctAnswer, llmUsed, feedback) // Set feedback to default (0)

	if err != nil {
		log.Printf("Failed to insert question: %v", err)
	}
}


// Handle user feedback (like/dislike)
func handleFeedback(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Parse feedback from the request
	var feedback struct {
		QuestionID int `json:"question_id"`
		Feedback   int `json:"feedback"`
	}

	// Decode the feedback
	if err := json.NewDecoder(r.Body).Decode(&feedback); err != nil {
		http.Error(w, "Failed to parse feedback", http.StatusBadRequest)
		return
	}

	// Update feedback in the database
	_, err := db.Exec("UPDATE prompts SET feedback = ? WHERE id = ?", feedback.Feedback, feedback.QuestionID)
	if err != nil {
		http.Error(w, "Failed to update feedback", http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Feedback submitted successfully",
	})
}

// Fetch history of generated questions
func handleHistory(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	rows, err := db.Query("SELECT id, prompt, context, question, options, correct_answer, llm_used FROM prompts")
	if err != nil {
		http.Error(w, "Failed to fetch history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []Question
	for rows.Next() {
		var q Question
		if err := rows.Scan(&q.ID, &q.Prompt, &q.Context, &q.QuestionText, &q.Options, &q.CorrectAnswer, &q.LLMUsed); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}

		// q.Options is already a slice of strings, so don't split it
		// The options should already be properly represented as a slice of strings
		// You can use the slice directly, no need to call strings.Split here

		history = append(history, q)
	}

	// Send the history response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(history)
}

