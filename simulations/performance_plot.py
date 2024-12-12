import re
import matplotlib.pyplot as plt
from datetime import datetime
import numpy as np
import pandas as pd
import sys
import csv
import os

FOLDER_MAPPING = {
    "singlePerformance": "results/singlePerformance_logs",
    "singlePerformance_parallelized": "results/singlePerformance_logs_parallel",
    "preprocessingPerformance": "results/prepoPerformance_logs",
    "preprocessingPerformance_parallelized": "results/prepoPerformance_logs_parallelized"
}


def sort_ram_usage():
    file_path = 'results/singlePerformance_logs_parallel/elliptic_ram_usage.txt'
    output_path = 'results/singlePerformance_logs_parallel/elliptic_ram_usage_cleaned.csv'

    with open(file_path, 'r') as file:
        lines = file.readlines()

    results = []  # Liste für die sortierten Paare
    start_buffer = {}  # Buffer für Start-Einträge, sortiert nach Repetition

    for line_number, line in enumerate(lines):
        line = line.strip()  # Entferne unnötige Leerzeichen
        if not line:
            continue  # Überspringe leere Zeilen

        try:
            # Extrahiere relevante Informationen aus der Zeile
            match = re.match(r"(Start|End) of repetition (\d+): RAM Usage: (\d+) MB", line)
            if not match:
                raise ValueError(f"Ungültiges Zeilenformat: {line}")

            action, repetition, ram_usage = match.groups()
            repetition = int(repetition)
            ram_usage = int(ram_usage)

            # Verarbeite Start-Einträge
            if action == "Start":
                if repetition not in start_buffer:
                    start_buffer[repetition] = []
                start_buffer[repetition].append((line, ram_usage, line_number))

            # Verarbeite End-Einträge
            elif action == "End":
                if repetition in start_buffer and start_buffer[repetition]:
                    # Nimm das erste Start-Paar aus dem Buffer
                    start_line, start_ram, start_line_number = start_buffer[repetition].pop(0)
                    results.append((start_line, line, start_line_number, line_number))
        except Exception as e:
            print(f"Fehler beim Verarbeiten der Zeile: {line}, Fehler: {e}")

    # Sortiere die Ergebnisse nach Zeilennummern
    results.sort(key=lambda x: (x[2], x[3]))

    # Schreibe die Ergebnisse in die Ausgabedatei
    with open(output_path, 'w') as file:
        for start, end, _, _ in results:
            file.write(f"{start}\n")
            file.write(f"{end}\n")

def sort_CPU_usage():
    input_csv = 'results/singlePerformance_logs_parallel/elliptic_cpu_usage_max.csv'
    output_csv = 'results/singlePerformance_logs_parallel/elliptic_cpu_usage_sorted.csv'
    # Lade die CSV-Datei ein
    with open(input_csv, "r") as infile:
        reader = csv.reader(infile)
        header = next(reader)  # Kopfzeile überspringen
        rows = list(reader)

    # Liste für die sortierten Paare
    sorted_pairs = []

    # Set für bereits besuchte Zeilen
    visited_indices = set()

    # Gehe zeilenweise durch die Daten
    for i, row in enumerate(rows):
        print(i)
        print(row)
        if i in visited_indices:
            continue

        current_repetition = row[0]
        current_value = row[1]

        # Suche nach der nächsten Zeile mit derselben Repetition-Nummer
        for j in range(i + 1, len(rows)):
            if j in visited_indices:
                continue

            next_repetition = rows[j][0]
            next_value = rows[j][1]

            if current_repetition == next_repetition:
                # Füge das Paar zu den sortierten Daten hinzu
                sorted_pairs.append([current_repetition, current_value])
                sorted_pairs.append([next_repetition, next_value])

                # Markiere die Zeilen als besucht
                visited_indices.add(i)
                visited_indices.add(j)

                break  # Breche die innere Schleife ab, sobald ein Paar gefunden wurde

    # Schreibe die sortierten Paare in die Ausgabedatei
    with open(output_csv, "w", newline="") as outfile:
        writer = csv.writer(outfile)
        writer.writerow(["Repetition", "Max_CPU_Usage_us"])
        writer.writerows(sorted_pairs)

def convert_to_ms(time_str):
    """Konvertiert Zeitangaben in Millisekunden."""
    if match := re.match(r"([0-9]+\.[0-9]+)ms", time_str):
        return float(match.group(1))  # Zeit bereits in Millisekunden
    elif match := re.match(r"([0-9]+\.[0-9]+)s", time_str):
        return float(match.group(1)) * 1000  # Zeit in Sekunden -> ms
    elif match := re.match(r"([0-9]+)m([0-9]+\.[0-9]+)s", time_str):
        minutes = int(match.group(1))
        seconds = float(match.group(2))
        return (minutes * 60 + seconds) * 1000  # Minuten und Sekunden -> ms
    else:
        raise ValueError(f"Ungültiges Zeitformat: {time_str}")


def clean_CPU_data():
    input_file = 'results/singlePerformance_logs_parallel/elliptic_cpu_usage.txt'
    output_file = 'results/singlePerformance_logs_parallel/elliptic_cpu_usage_cleaned.csv'
    iteration_numbers = []

    # Regex-Muster zum Erkennen von "Start of ..." oder "End of ..."
    pattern = r"^(?:Start|End) of (?:repition|repetition) (\d+):"

    # Eingabedatei zeilenweise lesen
    with open(input_file, "r") as file:
        for line in file:
            match = re.match(pattern, line.strip())
            if match:
                iteration_numbers.append(int(match.group(1)))

    # Ergebnisse in die CSV-Datei schreiben
    with open(output_file, "w", newline="") as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(["Iteration"])
        for number in iteration_numbers:
            writer.writerow([number])

    print(f"CSV-Datei wurde erfolgreich erstellt: {output_file}")

    output_file= 'results/singlePerformance_logs_parallel/elliptic_cpu_usage_max.csv'

    pattern = r"(?:(?:Start|End) of (?:repetition|repition) (\d+): .*Cpu\(s\):\s+(\d+\.\d+)\s+us.*)|(?:%Cpu\(s\):\s+(\d+\.\d+)\s+us.*)"
    repetitions = []
    max_cpu_usages = []
    current_repetition = None
    max_cpu = 0.0

    with open(input_file, 'r') as file:
        for line in file:
            match = re.match(pattern, line)
            if match:
                if match.group(1):  # Neue Repetition erkannt
                    if current_repetition is not None:
                        # Aktuelle Repetition abschließen
                        repetitions.append(current_repetition)
                        max_cpu_usages.append(max_cpu)

                    # Neue Repetition starten
                    current_repetition = int(match.group(1))
                    max_cpu = float(match.group(2))
                elif match.group(3):  # CPU-Wert innerhalb einer laufenden Repetition
                    max_cpu = max(max_cpu, float(match.group(3)))

        # Letzte Repetition hinzufügen
        if current_repetition is not None:
            repetitions.append(current_repetition)
            max_cpu_usages.append(max_cpu)

    # Ergebnisse in eine CSV-Datei schreiben
    with open(output_file, 'w', newline='') as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(['Repetition', 'Max_CPU_Usage_us'])
        for rep, cpu in zip(repetitions, max_cpu_usages):
            writer.writerow([rep, cpu])

    base_csv_file = "results/singlePerformance_logs/elliptic_cpu_usage_max.csv"
    second_csv_file = "results/singlePerformance_logs_parallel/elliptic_cpu_usage_sorted.csv"
    output_csv_file = "results/CPU_usage_elliptic.csv"

    # Daten aus der ersten CSV-Datei lesen
    with open(base_csv_file, "r") as base_file:
        base_reader = list(csv.reader(base_file))
        base_header = base_reader[0]
        base_data = base_reader[1:]

    # Daten aus der zweiten CSV-Datei lesen
    with open(second_csv_file, "r") as second_file:
        second_reader = list(csv.reader(second_file))
        second_header = second_reader[0]
        second_data = second_reader[1:]

    # Überprüfen, ob die Anzahl der Zeilen übereinstimmt
    if len(base_data) != len(second_data):
        raise ValueError("Die beiden CSV-Dateien haben unterschiedliche Zeilenanzahlen.")

    # Neue Spalte aus der zweiten CSV-Datei hinzufügen
    merged_data = [base_header + ["Second_Max_CPU_Usage"]]
    for base_row, second_row in zip(base_data, second_data):
        merged_data.append(base_row + [second_row[1]])

    # Zusammengeführte Daten in eine neue CSV-Datei schreiben
    with open(output_csv_file, "w", newline="") as output_file:
        writer = csv.writer(output_file)
        writer.writerows(merged_data)

    print(f"Zusammenführung abgeschlossen. Ergebnis gespeichert in {output_csv_file}")


def clean_RAM_data():
    txt_file = "results/singlePerformance_logs_parallel/elliptic_ram_usage.txt"  # Die .txt-Datei
    csv_file = "results/singlePerformance_logs_parallel/elliptic_ram_usage_cleaned.csv"  # Die bestehende CSV-Datei
    output_file = "results/elliptic_ram_usage_parallel.csv"


    with open(txt_file, "r") as txt, open(csv_file, "w", newline="") as csv_out:
        writer = csv.writer(csv_out)

        for line in txt:
            # Schreibe jede Zeile aus der TXT-Datei unverändert in die CSV-Datei
            writer.writerow([line.strip()])

    cleaned_data = []

    # Datei einlesen und Zeilen verarbeiten
    with open(csv_file, "r") as file:
        for line in file:
            # Muster zum Extrahieren von Repetition-Nummer und RAM-Wert
            match = re.match(r"^(?:Start|End) of (?:repition|repetition) (\d+): RAM Usage: (\d+) MB", line)
            print(match)
            if match:
                repetition = int(match.group(1))
                ram_usage = int(match.group(2))
                cleaned_data.append([repetition, ram_usage])

    # Bereinigte Daten in eine CSV-Datei schreiben
    with open(output_file, "w", newline="") as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(["Repetition", "RAM_Usage_MB"])
        writer.writerows(cleaned_data)

    print(f"Die Datei wurde erfolgreich in {csv_file} konvertiert.")

def clean_Exec_data():
    file1 = 'results/singlePerformance_logs/elliptic_execution_time.txt'
    file2 = 'results/singlePerformance_logs_parallel/elliptic_execution_time.txt'
    output_file = 'results/elliptic_execution_time.csv'
    # Beide Dateien lesen
    with open(file1, "r") as f1, open(file2, "r") as f2:
        lines1 = [line.strip() for line in f1.readlines() if "Execution Time:" in line]
        lines2 = [line.strip() for line in f2.readlines() if "Execution Time:" in line]

    if len(lines1) != len(lines2):
        raise ValueError("Die Dateien haben nicht die gleiche Anzahl an Einträgen.")

    # Normalisierte Zeiten extrahieren
    times1 = [convert_to_ms(line.split(":")[1].strip()) for line in lines1]
    times2 = [convert_to_ms(line.split(":")[1].strip()) for line in lines2]

    # Ergebnisse in einer CSV speichern
    with open(output_file, "w", newline="") as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(["Repetition", "Execution_Time_File1_ms", "Execution_Time_File2_ms"])

        for repetition, (time1, time2) in enumerate(zip(times1, times2), start=1):
            writer.writerow([repetition, time1, time2])


def plot_CPU_elliptic():
    pass


def plot_RAM_elliptic():
    pass


def plot_Exec_elliptic():
    pass


def plot_CPU_LWE():
    pass


def plot_RAM_LWE():
    pass


def plot_Exec_LWE():
    pass


def plot_CPU_LWE128():
    pass


def plot_RAM_LWE128():
    pass


def plot_Exec_LWE128():
    pass


def plot_CPU_merkle():
    pass


def plot_RAM_merkle():
    pass


def plot_Exec_merkle():
    pass


if __name__ == '__main__':
    clean_CPU_data()
    if len(sys.argv) < 2:
        print("Usage: python performance_plot.py <key>")
        print("Available options: ")
        for key in FOLDER_MAPPING.keys():
            print(f" -  {key}")
        sys.exit(1)

    folder_key = sys.argv[1]
    folder_path = FOLDER_MAPPING.get(folder_key)

    if not folder_path:
        print(f"Invalid folder key: {folder_key}")
        sys.exit(1)

    print(f"Evaluate performance for folder: {folder_path}")

    if folder_key == "singlePerformance" or folder_key == "singlePerformance_parallelized":
        pass
    elif folder_key == "preprocessingPerformance" or folder_key == "preprocessingPerformance_parallelized":
        pass
