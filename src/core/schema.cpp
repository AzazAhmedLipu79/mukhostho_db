#ifdef _WIN32
#include <windows.h> // For CreateDirectory
#else
#include <sys/stat.h> // For mkdir on POSIX
#include <sys/types.h>
#endif

#include <iostream>
#include <string>
#include <fstream> // For file streams
#include "schema.h"
#include "../util/json.hpp"


using namespace std;


string getFieldTypeName(FieldType type) {
    switch (type) {
        case FieldType::INT: return "int";
        case FieldType::FLOAT: return "float";
        case FieldType::BOOL: return "bool";
        case FieldType::STRING: return "string";
        case FieldType::ARRAY_STRING: return "array_string";
        case FieldType::ARRAY_INT: return "array_int";
        case FieldType::ARRAY_FLOAT: return "array_float";
        case FieldType::ARRAY_BOOL: return "array_bool";
        default: return "unknown";
    }
}


void printSchema(const Schema& schema) {
    cout << "Collection: " << schema.collectionName << endl;
    cout << "Fields:" << endl;
    for (const auto& field : schema.fields) {
        cout << "  - " << field.name << " (" << getFieldTypeName(field.type);
        if (field.type == FieldType::STRING || static_cast<int>(field.type) >= static_cast<int>(FieldType::ARRAY_STRING)) {
            cout << " <= " << field.maxLength;
        }
        cout << ")" << endl;
    }
}


void createSchema(Schema& schema) {
    cout << "Enter collection name: ";
    cin >> schema.collectionName;

    int fieldCount;
    cout << "Enter the number of fields: ";
    cin >> fieldCount;

    schema.fieldsCount = fieldCount;

    for (int i = 0; i < fieldCount; ++i) {
        Field field;
        cout << "Enter name of field " << i + 1 << ": ";
        cin >> field.name;

        int fieldType;
        cout << "Select type (0: int, 1: float, 2: bool, 3: string, "
                "4: array_string, 5: array_int, 6: array_float, 7: array_bool): ";
        cin >> fieldType;
        field.type = static_cast<FieldType>(fieldType);

        if (field.type == FieldType::STRING || static_cast<int>(field.type) >= static_cast<int>(FieldType::ARRAY_STRING)) {
            cout << "Enter max length for this field: ";
            cin >> field.maxLength;
        } else {
            field.maxLength = 0;
        }

        schema.fields.push_back(field);
    }

    cout << "Schema created successfully!" << endl;
    printSchema(schema);
}


void saveSchemaToFile(const Schema& schema) {
    string directory = SCHEMA_PATH;
    
    // Create the directory
    #ifdef _WIN32
        if (!CreateDirectory(directory.c_str(), NULL) && GetLastError() != ERROR_ALREADY_EXISTS) { 
            cout << directory.c_str() <<endl;
            cerr << "Error creating schemas directory." << endl;
            return;
        }
    #else
        if (mkdir(directory.c_str(), 0777) == -1 && errno != EEXIST) {
            cerr << "Error creating schemas directory." << endl;
            return;
        }
    #endif

        string filename = directory + schema.collectionName + ".schema.json";
        ofstream file(filename);

        if (!file) {
            cerr << "Failed to open file for writing: " << filename << endl;
            return;
        }

        // Write the schema to the file (example structure)
        file << "{\n";
        file << "  \"collectionName\": \"" << schema.collectionName << "\",\n";
        file << "  \"fieldsCount\": " << schema.fieldsCount << ",\n";
        file << "  \"fields\": [\n";
        for (size_t i = 0; i < schema.fields.size(); ++i) {
            const Field& field = schema.fields[i];
            file << "    {\n";
            file << "      \"name\": \"" << field.name << "\",\n";
            file << "      \"type\": \"" << getFieldTypeName(field.type) << "\"";
            if (field.type == FieldType::STRING || static_cast<int>(field.type) >= static_cast<int>(FieldType::ARRAY_STRING)) {
                file << ",\n      \"max_length\": " << field.maxLength;
            }
            file << "\n    }";
            if (i < schema.fields.size() - 1) {
                file << ",";
            }
            file << "\n";
        }
        file << "  ]\n";
        file << "}\n";

        file.close();
        cout << "Schema saved successfully to: " << filename << endl;
}


// NEED TO FIX IT
bool loadSchemaFromFile(Schema& schema, const string& collectionName) {
    string filename = SCHEMA_PATH + collectionName + ".schema.json"; 
    ifstream file(filename);
    if (!file.is_open()) {
        cerr << "Failed to open schema file: " << filename << endl;
        
        // Implement available schemas suggestion here

        return false;
    }

    schema.collectionName = collectionName;

    cout << "Loading schema from file: " << filename << endl;

    cout << "File content: " << endl;


    string temp; 
    string line;
   // Load the file content into temp
        while (getline(file, line)) {
            temp += line + "\n"; // Append the line with a newline character
        }

        cout << temp << endl;
 
 
    // Parse the JSON content
    nlohmann::json jsonData = nlohmann::json::parse(temp); 

    // Extract the collection name
    schema.collectionName = jsonData["collectionName"];

    // Extract the fields count
    schema.fieldsCount = jsonData["fieldsCount"];

    // Extract the fields
    nlohmann::json fields = jsonData["fields"];

    // assign the fields to the schema
    for(int i = 0; i < schema.fieldsCount; i++) {
        Field field; 

        field.name = fields[i]["name"];
        string typeStr = fields[i]["type"];
        if (typeStr == "int") field.type = FieldType::INT;
        else if (typeStr == "float") field.type = FieldType::FLOAT;
        else if (typeStr == "bool") field.type = FieldType::BOOL;
        else if (typeStr == "string") field.type = FieldType::STRING;
        else if (typeStr == "array_string") field.type = FieldType::ARRAY_STRING;
        else if (typeStr == "array_int") field.type = FieldType::ARRAY_INT;
        else if (typeStr == "array_float") field.type = FieldType::ARRAY_FLOAT;
        else if (typeStr == "array_bool") field.type = FieldType::ARRAY_BOOL;
 
       // cout << fields[i]["max_length"] << endl;
        // // Handle the max_length
        //field.maxLength = fields[i]["max_length"] && stoi(fields[i]["max_length"]); 

        schema.fields.push_back(field);
    }






    // iterate over the fields and extract the fields
/* 
    for (int i = 0; i < jsonData['fieldsCount']; i++) {
        Field field; 

        cout << fields[i] << endl;
        cout << fields[i]["name"] << endl;
        cout << fields[i]["type"] << endl;
        cout << fields[i]["max_length"] << endl;

        // // Handle the name
        // field.name = fields[i]["name"];

        // // Handle the type
        // string typeStr = fields[i]["type"];
        // if (typeStr == "int") field.type = FieldType::INT;
        // else if (typeStr == "float") field.type = FieldType::FLOAT;
        // else if (typeStr == "bool") field.type = FieldType::BOOL;
        // else if (typeStr == "string") field.type = FieldType::STRING;
        // else if (typeStr == "array_string") field.type = FieldType::ARRAY_STRING;
        // else if (typeStr == "array_int") field.type = FieldType::ARRAY_INT;
        // else if (typeStr == "array_float") field.type = FieldType::ARRAY_FLOAT;
        // else if (typeStr == "array_bool") field.type = FieldType::ARRAY_BOOL;


        // // Handle the max_length
        // field.maxLength = field[i]["max_length"] && stoi(field[i]["max_length"]); 

        // schema.fields.push_back(field);
    }
      */

     

    return true;
 
 
    }

/* {
  "collectionName": "sessions",
  "fields": [
    {
      "name": "id",
      "type": "int"
    },
    {
      "name": "jwt_token",
      "type": "string",
      "max_length": 400
    },
    {
      "name": "created_at",
      "type": "int"
    }
  ]
} */





    
     