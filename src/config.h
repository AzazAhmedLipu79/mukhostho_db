#ifndef CONFIG_H
#define CONFIG_H 


#define MAX_NAME_LENGTH 100
#define COLUMN_MAX_NAME 100
#define MAX_STRING_LENGTH 255
#define SCHEMA_PATH "../data/schemas/"

// DATA TYPES 
struct StringData {
    char value[MAX_STRING_LENGTH];
};

struct IntData {
    int value;
};

struct BoolData {
    bool value;
};

struct FloatData {
    float value;
};

#endif