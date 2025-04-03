import os
import random
import time
from datetime import datetime
import shutil

def create_test_logs_folder():
    if not os.path.exists('test_logs'):
        os.makedirs('test_logs')

def create_initial_files():
    for i in range(100):
        filename = f'test_logs/log_{i}.txt'
        with open(filename, 'w') as f:
            f.write(f'Initial log file {i}\n')

def write_random_log(file_path):
    log_levels = ['INFO', 'WARNING', 'ERROR', 'DEBUG']
    messages = [
        'Processing request',
        'Database connection established',
        'Cache miss occurred',
        'User authentication successful',
        'File operation completed',
        'Network timeout detected',
        'Memory allocation failed',
        'Task completed successfully'
    ]
    
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    log_level = random.choice(log_levels)
    message = random.choice(messages)
    log_line = f'[{timestamp}] {log_level}: {message}\n'
    
    with open(file_path, 'a') as f:
        f.write(log_line)

def rotate_file(file_path):
    if os.path.exists(file_path):
        os.remove(file_path)
    with open(file_path, 'w') as f:
        f.write(f'Rotated file at {datetime.now()}\n')

def main():
    create_test_logs_folder()
    create_initial_files()
    
    start_time = time.time()
    end_time = start_time + 180
    
    while time.time() < end_time:
        # Get list of all log files
        log_files = [f for f in os.listdir('test_logs') if f.endswith('.txt')]
        
        if log_files:
            # Randomly choose an operation with weighted probabilities
            # 60% chance to write, ~13.3% chance for each other operation
            operation = random.choices(
                ['write', 'create', 'delete', 'rotate'],
                weights=[60, 13.3, 13.3, 13.3],
                k=1
            )[0]
            
            if operation == 'write':
                # Write to a random existing file
                file_to_write = random.choice(log_files)
                write_random_log(os.path.join('test_logs', file_to_write))
            
            elif operation == 'create':
                # Create a new file with a unique name
                new_file_num = max([int(f.split('_')[1].split('.')[0]) for f in log_files]) + 1
                new_file = f'test_logs/log_{new_file_num}.txt'
                with open(new_file, 'w') as f:
                    f.write(f'New log file created at {datetime.now()}\n')
            
            elif operation == 'delete':
                # Delete a random file
                file_to_delete = random.choice(log_files)
                os.remove(os.path.join('test_logs', file_to_delete))
            
            elif operation == 'rotate':
                # Rotate a random file
                file_to_rotate = random.choice(log_files)
                rotate_file(os.path.join('test_logs', file_to_rotate))
        
        # Small delay between operations
        time.sleep(0.001)
    
    # Pause for 5 seconds
    time.sleep(5)
    
    # Write incrementing numbers to all remaining files
    log_files = [f for f in os.listdir('test_logs') if f.endswith('.txt')]
    for i, log_file in enumerate(sorted(log_files)):
        with open(os.path.join('test_logs', log_file), 'a') as f:
            f.write(f'\nFinal number2: {i}\n')

if __name__ == '__main__':
    main()
