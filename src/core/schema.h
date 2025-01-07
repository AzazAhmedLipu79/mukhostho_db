#ifndef SCHEMA_H
#define SCHEMA_H
#include <string>
#include <vector>
using namespace std;


#include "../config.h"


enum class FieldType {
    INT,
    FLOAT,
    BOOL,
    STRING,
    ARRAY_STRING,
    ARRAY_INT,
    ARRAY_FLOAT,
    ARRAY_BOOL
};

// Field definition
struct Field {
    string name;
    FieldType type;
    int maxLength; // For strings or arrays, 0 otherwise
};


// Schema definition
struct Schema {
    string collectionName;
    int fieldsCount;
    vector<Field> fields;
};

// Record definition
struct Record {
    vector<void*> data; // Store pointers to field data
};


// Collection definition
struct Collection {
    Schema schema;
    // Placeholder for B-Tree (to be implemented later)
    void* btreeRoot = nullptr;
};

#endif