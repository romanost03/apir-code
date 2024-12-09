import re
import matplotlib.pyplot as plt
from datetime import datetime
import numpy as np

if __name__ == '__main__':
    # Raw data (replace this with your actual log data as a string)
    log_data = """
    2024/12/08 19:50:08 Merkle preprocessing evaluation for dbLen 8589934592 bits
    2024/12/08 19:56:30 Merkle preprocessing evaluation for dbLen 17179869184 bits
    2024/12/08 20:08:07 Merkle preprocessing evaluation for dbLen 25769803776 bits
    2024/12/08 20:25:01 Merkle preprocessing evaluation for dbLen 34359738368 bits
    2024/12/08 20:47:08 Merkle preprocessing evaluation for dbLen 42949672960 bits
    2024/12/08 21:15:15 Merkle preprocessing evaluation for dbLen 51539607552 bits
    2024/12/08 21:48:23 Merkle preprocessing evaluation for dbLen 60129542144 bits
    2024/12/08 22:26:36 Merkle preprocessing evaluation for dbLen 68719476736 bits
    2024/12/08 23:09:51 Merkle preprocessing evaluation for dbLen 77309411328 bits
    2024/12/09 00:00:33 Merkle preprocessing evaluation for dbLen 85899345920 bits
    2024/12/09 00:56:35 simulation terminated successfully.
    """

    # Regular expressions to extract timestamps and database sizes
    timestamp_pattern = re.compile(r"(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) Merkle preprocessing evaluation for dbLen (\d+) bits")

    # Parse the log data
    timestamps = []
    db_sizes = []

    for match in timestamp_pattern.finditer(log_data):
        timestamp, db_size = match.groups()
        timestamps.append(timestamp)
        db_sizes.append(int(db_size) / (8 * 1024**3))  # Convert bits to GiB

    db_sizes.append(int(10))
    timestamps.append(str('2024/12/09 00:56:35'))
    # Calculate processing times in seconds
    def parse_timestamp(ts):
        return datetime.strptime(ts, "%Y/%m/%d %H:%M:%S")

    times = [parse_timestamp(ts) for ts in timestamps]
    processing_durations = [(t2 - t1).total_seconds() for t1, t2 in zip(times[:-1], times[1:])]

    # Plot the results
    plt.figure(figsize=(10, 6))
    plt.plot(db_sizes[:-1], processing_durations, marker='o', label="Processing Time")
    plt.title("Processing Time vs Database Size")
    plt.xlabel("Database Size (GiB)")
    plt.ylabel("Processing Time (seconds)")
    plt.grid(True)
    plt.legend()
    plt.show()

    # Data from the table, measurements of CPU-usage
    minutes = [1, 15, 30, 45, 60, 75, 90, 105, 120, 135, 150, 165, 180, 195, 210, 225, 240, 255, 270, 285, 300, 375]
    cpu_usage = [
        0.68, 1.06, 1.03, 1.00,
        1.02, 1.01, 1.04,
        1.05, 1.05, 1.03,
        1.01, 1.01, 1.00,
        1.02, 1.01, 1.00,
        1.00, 1.04, 1.00,
        1.00, 1.01, 1.01
    ]

    # Plotting
    plt.figure(figsize=(10, 6))
    plt.plot(minutes, cpu_usage, marker='o', linestyle='-', label="CPU Usage (%)")

    # Labels and title
    plt.title("CPU Usage Over Time", fontsize=14)
    plt.xlabel("Time (minutes)", fontsize=12)
    plt.ylabel("CPU Usage (%)", fontsize=12)
    plt.grid(True)
    plt.legend()
    plt.tight_layout()

    # Show the plot
    plt.show()

    # Data from the table of main memory usage (in GB)
    repetitions = np.arange(1, 11)  # Repetition numbers (1 to 10)
    ram_start = [7.45, 12.5, 21, 25, 39, 39, 46, 52, 61, 68]  # Start RAM usage in GB
    ram_end = [28, 57, 88, 64, 157, 179, 213, 234, 286, 326]  # End RAM usage in GB

    # Plot
    plt.figure(figsize=(10, 6))
    plt.plot(repetitions, ram_start, label='RAM Start (GB)', marker='o')
    plt.plot(repetitions, ram_end, label='RAM End (GB)', marker='o')

    # Labels and legend
    plt.title('RAM Usage During Repetitions')
    plt.xlabel('Repetition Number')
    plt.ylabel('RAM Usage (GB)')
    plt.xticks(repetitions)
    plt.legend()

    # Show plot
    plt.grid(True)
    plt.tight_layout()
    plt.show()

    print(f"Peak main memory usage of: {max(ram_end)} GB.")
    print(f"")
