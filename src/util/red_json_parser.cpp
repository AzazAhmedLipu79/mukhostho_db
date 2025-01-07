/* 
JSON Parser
- Parse JSON string to JSON object
 */ 

#include <iostream>
#include <map>
#include <string>
#include <cctype>

using namespace std;

// Function to parse a simple JSON string and return a map of key-value pairs
map<string, string> parseJson(const string& jsonStr) {
    cout << "Parsing JSON data..." << endl;
    map<string, string> jsonData;
    size_t pos = 0;

    cout << "JSON string: " << jsonStr << endl;
    cout << "JSON string length: " << jsonStr.length() << endl;

    while (pos < jsonStr.length()) { 
        // Skip spaces or commas
        if (isspace(jsonStr[pos]) || jsonStr[pos] == ',') {
            ++pos;
            continue;
        }

        // Start of key (after opening quote)
        if (jsonStr[pos] == '"') {
            size_t startKey = pos + 1;
            size_t endKey = jsonStr.find('"', startKey);
            string key = jsonStr.substr(startKey, endKey - startKey);
            pos = endKey + 2;  // Skip the closing quote and colon

            // Start of value (after value starts with quote)
            if (jsonStr[pos] == '"') {
                size_t startValue = pos + 1;
                size_t endValue = jsonStr.find('"', startValue);
                string value = jsonStr.substr(startValue, endValue - startValue);
                jsonData[key] = value;  // Store in the map
                pos = endValue + 1;  // Skip closing quote
            }
        }
    }

    cout << "Parsed JSON data:" << endl;
    return jsonData;
}

// Function to display parsed JSON data
void displayJsonData(const map<string, string>& jsonData) {
    for (const auto& pair : jsonData) {
        cout << pair.first << ": " << pair.second << endl;
    }
}