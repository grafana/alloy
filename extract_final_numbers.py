import os
import re

def extract_final_numbers(file_path):
    total = 0
    pattern = r"Final number2: (\d+)"
    
    try:
        with open(file_path, 'r') as f:
            for line in f:
                match = re.search(pattern, line)
                if match:
                    number = int(match.group(1))
                    print(f"Final number2: {number}")
                    total += 1
    except Exception as e:
        print(f"Error reading file: {str(e)}")
    
    return total

if __name__ == "__main__":
    # Process the single log file
    log_file = "t.txt"
    
    if not os.path.exists(log_file):
        print(f"File {log_file} not found!")
        exit(1)
    
    print("Extracting Final numbers from log file...")
    print("-" * 50)
    
    total = extract_final_numbers(log_file)
    
    print("-" * 50)
    print(f"Total of all Final numbers: {total}") 