package main

import (
	"crypto/sha256" // Import sha256 package
	"database/sql"
	"encoding/hex" // Import hex encoding package
	"encoding/json"
	"fmt"
	"io" // Import io for reading response body on error
	"math/rand/v2"
	"net/http"
	"sort"
	"strings"
	"time" // Import time for seeding random number generator

	_ "github.com/lib/pq" // Add back the PostgreSQL driver import
)

const maxCodeLength = 65535 // Define a max length threshold (e.g., 64KB - 1)

// Create a string containing the given number of characters and UTF-8 encoded bytes.
func makeCode(chars *int, bytes int) (result string) {
	// Check if the requested byte size exceeds the defined limit.
	if bytes > maxCodeLength {
		// fmt.Printf("Info: Requested bytes (%d) exceeds limit (%d). Returning placeholder.\n", bytes, maxCodeLength)
		return "-- CODE OMITTED DUE TO SIZE LIMIT --"
	}

	// Handle cases where only byte count matters or is non-zero when char count is zero/nil.
	// This specifically covers assembly (chars == nil) and cases where chars might be decoded as 0
	// but bytes are specified.
	if chars == nil || (*chars == 0 && bytes > 0) {
		if bytes < 0 {
			// Defensive check: Should not happen, but handle it.
			fmt.Printf("Warning: makeCode called with nil/zero chars and negative bytes (%d). Returning empty string.\n", bytes)
			return ""
		}
		// Generate exactly 'bytes' number of single-byte characters (spaces).
		return strings.Repeat(" ", bytes)
	}

	// If chars is non-nil but points to 0, and bytes is also 0, return empty string.
	if *chars == 0 && bytes == 0 {
		return ""
	}

	// If target bytes is less than target chars, it's impossible with single-byte or multi-byte chars.
	// This scenario shouldn't happen with valid code golf data but handle defensively.
	if bytes < *chars {
		// Fallback: return a string of 'a's with the target char count.
		// This won't match the byte count but avoids crashing.
		fmt.Printf("Warning: makeCode called with bytes (%d) < chars (%d). Returning %d spaces.\n", bytes, *chars, *chars)
		return strings.Repeat(" ", *chars)
	}

	// If bytes and chars are equal, just use single-byte characters.
	if bytes == *chars {
		return strings.Repeat(" ", *chars)
	}

	// Calculate the difference between bytes and characters needed.
	delta := bytes - *chars
	resultRunes := make([]rune, *chars) // Pre-allocate rune slice for efficiency

	// Fill with multi-byte characters first to cover the delta.
	multiByteChars := []rune("😃景£") // Example multi-byte characters (UTF-8: 4, 3, 2 bytes respectively)
	runeIndex := 0

	for _, replacement := range multiByteChars {
		if runeIndex >= *chars { // Stop if we've filled all character slots
			break
		}
		replacementLen := len(string(replacement))
		replacementDelta := replacementLen - 1 // Bytes added per character beyond the first byte

		if replacementDelta <= 0 { // Should not happen with these examples
			continue
		}

		// How many times can we use this character?
		// Limited by the remaining delta and remaining character slots.
		count := 0
		if replacementDelta > 0 {
			count = delta / replacementDelta
		}

		remainingChars := *chars - runeIndex
		if count > remainingChars {
			count = remainingChars // Don't exceed the total character count
		}

		// Add the character 'count' times
		for i := 0; i < count && runeIndex < *chars; i++ {
			resultRunes[runeIndex] = replacement
			runeIndex++
			delta -= replacementDelta // Decrease delta for each multi-byte char used
		}
	}

	// Fill any remaining character slots with single-byte space ' '.
	for runeIndex < *chars {
		resultRunes[runeIndex] = ' '
		runeIndex++
	}

	// Convert the rune slice to a string.
	result = string(resultRunes)

	// Final check and adjustment if byte count is off.
	// This prioritizes matching the char count exactly.
	currentBytes := len(result)
	if currentBytes != bytes && *chars > 0 {
		// fmt.Printf("Info: Adjusting byte count. Initial: %d bytes, %d chars. Target: %d bytes. Delta: %d\n", currentBytes, *chars, bytes, bytes-currentBytes)

		// If bytes are too low, replace spaces with '£' (2 bytes)
		if currentBytes < bytes {
			neededBytes := bytes - currentBytes
			replacedCount := 0
			for i := len(resultRunes) - 1; i >= 0 && neededBytes > 0; i-- {
				if resultRunes[i] == ' ' {
					resultRunes[i] = '£'
					neededBytes-- // We added 1 byte (2 bytes for £ vs 1 for space)
					replacedCount++
				}
			}
			result = string(resultRunes)
			// fmt.Printf("Info: Adjusted bytes (low). Replaced %d spaces with £. Final bytes: %d\n", replacedCount, len(result))
		} else if currentBytes > bytes {
			// If bytes are too high, replace multi-byte chars with spaces
			neededToRemove := currentBytes - bytes
			replacedCount := 0
			for i := 0; i < len(resultRunes) && neededToRemove > 0; i++ {
				char := resultRunes[i]
				charLen := len(string(char))
				if charLen > 1 { // If it's a multi-byte character
					bytesSaved := charLen - 1 // Bytes saved by replacing with a space
					if bytesSaved > 0 {
						resultRunes[i] = ' '
						neededToRemove -= bytesSaved
						replacedCount++
					}
				}
			}
			result = string(resultRunes)
			// fmt.Printf("Info: Adjusted bytes (high). Replaced %d multi-byte chars. Final bytes: %d\n", replacedCount, len(result))

			// If still too high (e.g., only single bytes left), truncate (though this violates char count)
			if len(result) > bytes {
				// fmt.Printf("Warning: Could not fully reduce byte count. Truncating result string.\n")
				result = string([]rune(result)[:bytes]) // Crude truncation by runes, might not match bytes exactly
				// A better approach would be byte-level slicing, but it's complex.
				// Re-check length after rune slicing
				for len(result) > bytes {
					 runes := []rune(result)
					 result = string(runes[:len(runes)-1])
				}
			}
		}
	}

	// Final absolute check (should ideally not be needed often if logic above is sound)
	if len(result) != bytes || len([]rune(result)) != *chars {
	    // fmt.Printf("Warning: makeCode final check failed for chars=%d, bytes=%d. Result: %d runes, %d bytes. Code: %q\n", *chars, bytes, len([]rune(result)), len(result), result)
	    // If the check fails, especially for bytes, maybe return a simple placeholder
	    // return strings.Repeat("?", *chars) // Example fallback
	}


	return result
}

// Test function for makeCode
func testMakeCode() {
	fmt.Println("Testing makeCode...")
	testCases := []struct {
		chars *int
		bytes int
		name  string
	}{
		{pointInt(5), 5, "Simple ASCII"},
		{pointInt(5), 8, "Mixed 1 (2-byte)"}, // £££
		{pointInt(5), 10, "Mixed 2 (3,2-byte)"}, // 景£
		{pointInt(10), 15, "Longer Mixed 1"},
		{pointInt(10), 25, "Longer Mixed 2"},
		{pointInt(0), 0, "Empty String"},
		{pointInt(1), 4, "Single 4-byte char"}, // 😃
		{pointInt(2), 5, "One 3-byte, one 1-byte"}, // 景
		{pointInt(2), 6, "One 4-byte, one 2-byte"}, // 😃£
		{pointInt(3), 7, "One 4-byte, two 1-byte"}, // 😃
		{pointInt(3), 8, "One 4-byte, one 2-byte, one 1-byte"}, // 😃£
		{nil, 10, "Assembly (nil chars)"},
		{nil, 0, "Assembly (nil chars, 0 bytes)"},
		{pointInt(5), 4, "Impossible (bytes < chars)"}, // Test defensive case
		{pointInt(7), 8, "Tricky Case 1"}, // £
		{pointInt(7), 10, "Tricky Case 2"}, // £££
	}

	allPassed := true
	for _, tc := range testCases {
		result := makeCode(tc.chars, tc.bytes)
		runeCount := len([]rune(result))
		byteCount := len(result)

		expectedChars := 0
		if tc.chars != nil {
			expectedChars = *tc.chars
		} else {
			// For nil chars (assembly), rune count isn't fixed by input, only byte count matters.
			// The function generates 'bytes' single-byte spaces.
			expectedChars = tc.bytes // Expect 'bytes' runes because we use spaces.
		}

		// Adjust expectation for the impossible case (bytes < chars)
		if tc.chars != nil && tc.bytes < *tc.chars {
			expectedChars = *tc.chars // Expecting the fallback char count
			if runeCount != expectedChars {
				fmt.Printf("FAIL [%s]: makeCode(%v, %d) => Expected %d runes (fallback), got %d runes. String: %q\n",
					tc.name, tc.chars, tc.bytes, expectedChars, runeCount, result)
				allPassed = false
			} else {
				// fmt.Printf("PASS [%s]: makeCode(%v, %d) => Correctly handled impossible case (returned %d runes).\n", tc.name, tc.chars, tc.bytes, runeCount)
			}
			continue // Skip further checks for this case
		}

		// Standard checks for achievable cases
		// For nil chars, only check byte count
		checkChars := tc.chars != nil
		if (checkChars && runeCount != expectedChars) || byteCount != tc.bytes {
			charsVal := "nil"
			if tc.chars != nil {
				charsVal = fmt.Sprintf("%d", *tc.chars)
			}
			fmt.Printf("FAIL [%s]: makeCode(chars=%s, bytes=%d) => Expected %d runes/%d bytes, got %d runes/%d bytes. String: %q\n",
				tc.name, charsVal, tc.bytes, expectedChars, tc.bytes, runeCount, byteCount, result)
			allPassed = false
		} else {
			// fmt.Printf("PASS [%s]: makeCode(chars=%v, bytes=%d) => %d runes, %d bytes.\n", tc.name, tc.chars, tc.bytes, runeCount, byteCount)
		}
	}

	if !allPassed {
		panic("makeCode tests failed")
	}
	fmt.Println("makeCode tests passed.")
}


// Helper to get a pointer to an int literal
func pointInt(v int) *int {
	return &v
}

// Fetches the latest submission timestamp from the database.
func getLatestTimestamp(db *sql.DB) (result string) {
	var value sql.NullString // Use sql.NullString for potentially NULL values
	// Use COALESCE to return an empty string if max(submitted) is NULL (e.g., empty table)
	if err := db.QueryRow(`SELECT COALESCE(max(submitted)::text, '') FROM solutions`).Scan(&value); err != nil {
		fmt.Println("Error fetching latest timestamp:", err)
		panic(err)
	}

	if value.Valid {
		result = value.String
	}
	// fmt.Println("Latest timestamp from DB:", result)
	return
}

// Fetches existing users and maps their login names to IDs.
func getUserMap(db *sql.DB) (result map[string]int32) {
	result = map[string]int32{}
	fmt.Println("Fetching user map...")
	rows, err := db.Query(`SELECT id, login FROM users`)
	if err != nil {
		fmt.Println("Error querying users:", err)
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int32
		var login string

		if err := rows.Scan(&id, &login); err != nil {
			fmt.Println("Error scanning user row:", err)
			panic(err)
		}
		result[login] = id
	}

	if err := rows.Err(); err != nil {
		fmt.Println("Error iterating user rows:", err)
		panic(err)
	}
	fmt.Printf("Fetched %d users.\n", len(result))
	return
}

// Generates a random int32 User ID that is not already in use.
func getUnusedUserID(userMap map[string]int32) (result int32) {
	existingIDs := make(map[int32]bool)
	for _, userID := range userMap {
		existingIDs[userID] = true
	}

	// Seed the random number generator only once
	// Note: Using math/rand/v2 which doesn't require explicit seeding globally.
	// If using math/rand, you'd seed with rand.Seed(time.Now().UnixNano())

	for {
		result = rand.Int32()
		// Ensure the generated ID is positive and not zero, if your DB requires positive IDs
		if result <= 0 {
			continue
		}
		if _, exists := existingIDs[result]; !exists {
			return // Found an unused ID
		}
	}
}

// Calculates a simple SHA-256 hash for the language name.
func calculateLangDigest(lang string) string {
	hasher := sha256.New()
	hasher.Write([]byte(lang)) // Hash the language string
	return hex.EncodeToString(hasher.Sum(nil))
}

func main() {
	// Seed random number generator (important if using math/rand, less so for math/rand/v2)
	// rand.Seed(time.Now().UnixNano()) // Uncomment if using standard math/rand

	testMakeCode() // Run tests for the makeCode function

	// Establish database connection
	// Consider making connection string configurable (e.g., via env vars or flags)
	dbConnectionString := "user=code-golf sslmode=disable" // Replace with your actual connection string if needed
	db, err := sql.Open("postgres", dbConnectionString)
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		panic(err)
	}
	defer db.Close()

	// Ping DB to ensure connection is valid
	if err := db.Ping(); err != nil {
		fmt.Println("Error pinging database:", err)
		panic(err)
	}
	fmt.Println("Database connection successful.")

	// Fetch solution data from the Code Golf API
	apiURL := "https://code.golf/scores/all-holes/all-langs/all"
	fmt.Printf("Fetching data from Code Golf API (%s)...\n", apiURL)
	res, err := http.Get(apiURL)
	if err != nil {
		fmt.Println("Error fetching data from API:", err)
		panic(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		fmt.Printf("API request failed with status code: %d %s\n", res.StatusCode, res.Status)
		// Read and print the first part of the body for more details
		bodyBytes, _ := io.ReadAll(io.LimitReader(res.Body, 1024)) // Limit reading size
		fmt.Println("Response body (partial):", string(bodyBytes))
		panic(fmt.Errorf("API request failed: %s", res.Status))
	}

	// Define structure to decode JSON data
	var infoList []struct {
		Bytes                                 int
		Chars                                 *int   // Pointer to handle potential nulls (like for assembly)
		Hole, Lang, Login, Scoring, Submitted string // Timestamps are strings from API
	}

	// Decode the JSON response
	fmt.Println("Decoding JSON response...")
	if err := json.NewDecoder(res.Body).Decode(&infoList); err != nil {
		fmt.Println("Error decoding JSON response:", err)
		// It might be useful to know *where* in the JSON it failed, but standard library doesn't easily provide that.
		panic(err)
	}
	fmt.Printf("Fetched and decoded %d solution entries from API.\n", len(infoList))

	// Sort entries by submission time to process them chronologically
	// This helps ensure trophies are awarded based on the earliest submission.
	sort.Slice(infoList, func(i, j int) bool {
		// Consider adding secondary sort key (e.g., login, hole) for deterministic order on ties
		return infoList[i].Submitted < infoList[j].Submitted
	})
	fmt.Println("Sorted solution entries by submission time.")

	// Start a database transaction
	fmt.Println("Beginning database transaction...")
	tx, err := db.Begin()
	if err != nil {
		fmt.Println("Error beginning database transaction:", err)
		panic(err)
	}
	// Defer Rollback in case of panic, Commit will override this if successful
	defer func() {
		// Check if a panic occurred
		recovered := recover()
		// Always attempt rollback if a panic occurred OR if the commit failed (err != nil)
		if recovered != nil || err != nil {
			fmt.Println("Error or panic detected, attempting to rollback transaction...")
			if rbErr := tx.Rollback(); rbErr != nil {
				fmt.Printf("Error during transaction rollback: %v\n", rbErr)
			} else {
				fmt.Println("Transaction rolled back successfully.")
			}
		}
		// Re-panic if a panic was recovered
		if recovered != nil {
			panic(recovered)
		}
	}()


	// Get existing users from the database
	userMap := getUserMap(db) // Pass the db connection, not the transaction

	// Process users: Add any missing users from the API data to the DB within the transaction
	fmt.Println("Processing users...")
	usersAdded := 0
	// Prepare statement for user insertion
	userStmt, err := tx.Prepare(`INSERT INTO users (id, login) VALUES($1, $2) ON CONFLICT (login) DO NOTHING`)
	if err != nil {
		fmt.Println("Error preparing user insert statement:", err)
		panic(err) // Can't proceed without this statement
	}
	defer userStmt.Close()

	// Prepare statement for trophy insertion
	trophyStmt, err := tx.Prepare(`INSERT INTO trophies (earned, user_id, trophy) VALUES ($1, $2, 'hello-world') ON CONFLICT DO NOTHING`)
	if err != nil {
		fmt.Println("Error preparing trophy insert statement:", err)
		panic(err) // Can't proceed without this statement
	}
	defer trophyStmt.Close()

	// Use a map to track users processed in this run to avoid duplicate trophy inserts for the same user
	processedLogins := make(map[string]bool)

	for _, info := range infoList {
		// Check if user exists locally
		userID, userExists := userMap[info.Login]
		if !userExists {
			// Check if we already added this user in this run (handles duplicate logins in infoList)
			if _, processed := processedLogins[info.Login]; !processed {
				newUserID := getUnusedUserID(userMap)
				userMap[info.Login] = newUserID         // Add to local map immediately
				processedLogins[info.Login] = true // Mark as processed in this run
				userID = newUserID // Use the new ID for subsequent operations

				// Insert the new user using prepared statement
				if _, err = userStmt.Exec(userID, info.Login); err != nil { // Assign to outer err
					fmt.Printf("Error inserting user %s (ID: %d): %v\n", info.Login, userID, err)
					panic(err) // Stop processing if user insert fails (rollback handled by defer)
				}

				// Grant a default trophy using prepared statement
				if _, err = trophyStmt.Exec(info.Submitted, userID); err != nil { // Assign to outer err
					fmt.Printf("Error inserting trophy for user %s (ID: %d): %v\n", info.Login, userID, err)
					panic(err) // Stop processing if trophy insert fails (rollback handled by defer)
				}
				usersAdded++
			} else {
				// User was added earlier in this run, retrieve the ID
				userID = userMap[info.Login]
			}
		}
	}
	fmt.Printf("Processed users. Added %d new users.\n", usersAdded)

	// Get the timestamp of the most recent solution already in the database
	lastUpdateTime := getLatestTimestamp(db) // Get timestamp from DB before processing solutions
	updateCount := 0
	skippedCount := 0

	fmt.Println("Processing solutions...")
	// Prepare the statement for inserting/updating solutions for efficiency
	solutionStmt, err := tx.Prepare(
		`INSERT INTO solutions (bytes, chars, code, hole, lang, lang_digest, scoring, submitted, user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (user_id, hole, lang, scoring) DO UPDATE
		 SET bytes = excluded.bytes,
		     chars = excluded.chars,
		     code = excluded.code,
		     lang_digest = excluded.lang_digest, -- Also update lang_digest on conflict
		     submitted = excluded.submitted`)
	if err != nil {
		fmt.Println("Error preparing solution statement:", err)
		panic(err)
	}
	defer solutionStmt.Close()

	// Iterate through the fetched and sorted solutions
	startTime := time.Now()
	for i, info := range infoList {
		// Log progress periodically
		if (i+1)%10000 == 0 || i == len(infoList)-1 {
			elapsed := time.Since(startTime).Seconds()
			rate := float64(i+1) / elapsed
			if elapsed < 1 { // Avoid division by zero if very fast
				rate = float64(i+1)
			}
			fmt.Printf("Progress %d/%d (Skipped: %d, Updated: %d) [%.2f records/sec]\n",
				i+1, len(infoList), skippedCount, updateCount, rate)
		}

		// Skip solutions that are older than or the same age as the latest one in the DB
		if lastUpdateTime != "" && info.Submitted <= lastUpdateTime {
			skippedCount++
			continue // Ignore old solutions
		}

		// Generate the placeholder code string
		code := makeCode(info.Chars, info.Bytes)
		// Calculate the language digest
		langDigest := calculateLangDigest(info.Lang)
		// Get user ID from the map (should exist now after user processing step)
		userID, userExists := userMap[info.Login]
		if !userExists {
			// This should not happen if user processing was successful
			err = fmt.Errorf("user %s not found in map during solution processing for hole %s, lang %s", info.Login, info.Hole, info.Lang)
			fmt.Println("Error:", err)
			panic(err) // Stop processing (rollback handled by defer)
		}

		updateCount++

		// === DEBUGGING LOG ===
		// Add specific conditions if you only want to log the failing case,
		// or remove the condition to log all inserts/updates.
		// Example: Log only the previously failing case
		// if info.Lang == "assembly" && info.Hole == "fibonacci" && info.Login == "sisyphus-ppcg" {
		// Or log all assembly:
		// if info.Lang == "assembly" {
		// Or log everything (can be very verbose):
		if true { // Temporarily log all inserts/updates near the expected failure point for context
			// Limit logging frequency if needed
			if updateCount > 40000 && updateCount < 40100 { // Example: Log around where the error might occur
				fmt.Printf("DEBUG: Preparing to insert/update solution #%d (Overall index %d):\n", updateCount, i)
				fmt.Printf("  Login: %s (ID: %d)\n", info.Login, userID)
				fmt.Printf("  Hole: %s, Lang: %s, Scoring: %s\n", info.Hole, info.Lang, info.Scoring)
				fmt.Printf("  Bytes: %d\n", info.Bytes)
				charsValStr := "NULL"
				charsValPtr := "nil"
				if info.Chars != nil {
					charsValStr = fmt.Sprintf("%d", *info.Chars)
					charsValPtr = fmt.Sprintf("%p", info.Chars)
				}
				fmt.Printf("  Chars Ptr: %s\n", charsValPtr)
				fmt.Printf("  Chars Value: %s\n", charsValStr)
				fmt.Printf("  Generated Code: %q\n", code) // Quote the string to see if it's empty
				fmt.Printf("  Generated Code len(bytes): %d\n", len(code))
				fmt.Printf("  Generated Code len(runes): %d\n", len([]rune(code)))
				fmt.Printf("  Lang Digest: %s\n", langDigest)
				fmt.Printf("  Submitted: %s\n", info.Submitted)
			}
		}
		// === END DEBUGGING LOG ===

		// Cap byte and char counts if they exceed the threshold
		bytesValue := info.Bytes
		// For DB constraints, make sure the bytesValue matches the actual code length
		if bytesValue > maxCodeLength {
			// Don't just cap the value, make sure it matches the code length
			bytesValue = len(code)
		}

		// Convert *int to sql.NullInt32 for robust NULL handling AND cap the value
		var sqlChars sql.NullInt32
		if info.Chars != nil {
			// For DB constraints, make sure the char count matches the actual rune count of code
			charValue := len([]rune(code))
			sqlChars.Valid = true
			sqlChars.Int32 = int32(charValue) // Use actual count from code
		} else {
			sqlChars.Valid = false // Represents SQL NULL
		}

		// Execute the prepared statement for solutions
		_, err = solutionStmt.Exec( // Assign to outer err
			bytesValue, // Pass potentially capped bytes value
			sqlChars,   // Pass potentially capped sql.NullInt32
			code,
			info.Hole,
			info.Lang,
			langDigest, // Add the calculated lang_digest
			info.Scoring,
			info.Submitted,
			userID,
		)
		if err != nil {
			// Provide more context on error
			charsValStr := "NULL"
			if sqlChars.Valid { // Check Valid field of sql.NullInt32
				charsValStr = fmt.Sprintf("%d", sqlChars.Int32)
			}
			// Define originalCharsStr based on the original info.Chars
			originalCharsStr := "NULL"
			if info.Chars != nil {
				originalCharsStr = fmt.Sprintf("%d", *info.Chars)
			}
			fmt.Printf("Error executing solution statement for user %s (ID: %d), hole %s, lang %s, scoring %s\n",
				info.Login, userID, info.Hole, info.Lang, info.Scoring)
			fmt.Printf("  Values (Original API): bytes=%d, chars=%s, code=%q (len=%d), lang_digest=%s, submitted=%s\n",
				info.Bytes, originalCharsStr, code, len(code), langDigest, info.Submitted) // Log original API values
			fmt.Printf("  Values (Inserted):   bytes=%d, chars=%s\n",
				bytesValue, charsValStr) // Log inserted (potentially capped) values
			fmt.Printf("  Error details: %v\n", err) // Print the specific pq error

			panic(err) // Stop processing (rollback handled by defer)
		}
	}

	// Commit the transaction
	fmt.Println("Committing transaction...")
	err = tx.Commit() // Assign error from Commit to the 'err' variable
	if err != nil {
		fmt.Println("Error committing transaction:", err)
		// Rollback is handled by the defer function checking 'err'
		panic(err)
	}

	fmt.Printf("Processing complete. Total solutions processed: %d, Skipped: %d, Inserted/Updated: %d\n", len(infoList), skippedCount, updateCount)
}
