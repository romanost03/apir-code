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
    "preprocessingPerformance": "results/prepoPerformance_logs",
}


def sort_ram_usage(key, folder_key):
    file_path = None
    output_path = None
    if folder_key == "singlePerformance":
        file_path = f'results/singlePerformance_logs_parallel/{key}_ram_usage.txt'
        output_path = f'results/singlePerformance_logs_parallel/{key}_ram_usage_cleaned.csv'
    elif folder_key == "preprocessingPerformance":
        file_path = f'results/prepoPerformance_logs_parallelized/merkle_ram_usage.txt'
        output_path = f'results/prepoPerformance_logs_parallelized/merkle_ram_usage_cleaned.csv'

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
            match = re.match(r"(Start|End) of (?:repetition|repition|repitition) (\d+): RAM Usage: (\d+) MB", line)
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

def sort_CPU_usage(key, folder_key):
    input_csv = None
    output_csv = None
    if folder_key == "singlePerformance":
        input_csv = f'results/singlePerformance_logs_parallel/{key}_cpu_usage_max.csv'
        output_csv = f'results/singlePerformance_logs_parallel/{key}_cpu_usage_sorted.csv'
    elif folder_key == "preprocessingPerformance":
        input_csv = f'results/prepoPerformance_logs_parallelized/merkle_cpu_usage_max.csv'
        output_csv = f'results/prepoPerformance_logs_parallelized/merkle_cpu_usage_sorted.csv'

    # Lade die CSV-Datei ein
    rows = []  # Initialisiere eine Liste für die Daten
    with open(input_csv, "r") as infile:
        reader = csv.reader(infile)
        try:
            header = next(reader)  # Kopfzeile überspringen
        except StopIteration:
            raise ValueError(f"Die Datei {input_csv} ist leer oder ungültig.")  # Fehler bei leerer Datei

        for line in reader:
            if line:  # Überspringe leere Zeilen
                rows.append(line)

    if not rows:
        raise ValueError(f"Keine gültigen Daten in der Datei {input_csv} gefunden.")

    # Liste für die sortierten Paare
    sorted_pairs = []

    # Set für bereits besuchte Zeilen
    visited_indices = __builtins__.set()

    # Gehe zeilenweise durch die Daten
    for i, row in enumerate(rows):
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


def clean_CPU_data(key, parallel, folder_key):

    input_file = None
    output_file = None
    if folder_key == "singlePerformance":
        if parallel is True:
            input_file = f'results/singlePerformance_logs_parallel/{key}_cpu_usage.txt'
            output_file = f'results/singlePerformance_logs_parallel/{key}_cpu_usage_cleaned.csv'
        else:
            input_file = f'results/singlePerformance_logs/{key}_cpu_usage.txt'
            output_file = f'results/singlePerformance_logs/{key}_cpu_usage_cleaned.csv'
    elif folder_key == "preprocessingPerformance":
        if parallel is True:
            input_file = f'results/prepoPerformance_logs_parallelized/merkle_cpu_usage.txt'
            output_file = f'results/prepoPerformance_logs_parallelized/merkle_cpu_usage_cleaned.csv'
        else:
            input_file = f'results/prepoPerformance_logs/merkle_cpu_usage.txt'
            output_file = f'results/prepoPerformance_logs/merkle_cpu_usage_cleaned.csv'

    iteration_numbers = []

    # Regex-Muster zum Erkennen von "Start of ..." oder "End of ..."
    pattern = r"^(?:Start|End) of (?:repition|repetition|repitition) (\d+):"

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

    if folder_key == "singlePerformance":
        if parallel is True:
            output_file= f'results/singlePerformance_logs_parallel/{key}_cpu_usage_max.csv'
        else:
            output_file= f'results/singlePerformance_logs/{key}_cpu_usage_max.csv'
    elif folder_key == "preprocessingPerformance":
        if parallel is True:
            output_file= f'results/prepoPerformance_logs_parallelized/merkle_cpu_usage_max.csv'
        else:
            output_file= f'results/prepoPerformance_logs/merkle_cpu_usage_max.csv'

    pattern = r"(?:(?:Start|End) of (?:repetition|repition|repitition) (\d+): .*Cpu\(s\):\s+(\d+\.\d+)\s+us.*)|(?:%Cpu\(s\):\s+(\d+\.\d+)\s+us.*)"
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

def merge_CPU_Usage(key, folder_key):
    base_csv_file = None
    second_csv_file = None
    output_csv_file = None
    if folder_key == "singlePerformance":
        base_csv_file = f'results/singlePerformance_logs/{key}_cpu_usage_max.csv'
        second_csv_file = f'results/singlePerformance_logs_parallel/{key}_cpu_usage_sorted.csv'
        output_csv_file = f'results/CPU_usage_{key}.csv'
    elif folder_key == "preprocessingPerformance":
        base_csv_file = f'results/prepoPerformance_logs/merkle_cpu_usage_max.csv'
        second_csv_file = f'results/prepoPerformance_logs_parallelized/merkle_cpu_usage_sorted.csv'
        output_csv_file = f'results/CPU_usage_merkle.csv'

    # CSV-Dateien laden
    base_df = pd.read_csv(base_csv_file)
    second_df = pd.read_csv(second_csv_file)

    # Überprüfen, ob die Anzahl der Zeilen übereinstimmt
    if len(base_df) != len(second_df):
        raise ValueError("Die beiden CSV-Dateien haben unterschiedliche Zeilenanzahlen.")

    # Neue Spalte aus der zweiten Datei hinzufügen
    base_df["Second_Max_CPU_Usage_us"] = second_df["Max_CPU_Usage_us"]

    # Ergebnis in eine neue CSV-Datei speichern
    base_df.to_csv(output_csv_file, index=False)
    print(f"Zusammenführung abgeschlossen. Ergebnis gespeichert in {output_csv_file}")



def clean_RAM_data(key, folder_key):
    txt_file = None
    csv_file = None
    csv_file_2 = None
    output_file = None
    if folder_key == "singlePerformance":
        txt_file = f"results/singlePerformance_logs/{key}_ram_usage.txt"  # Die .txt-Datei
        csv_file = f"results/singlePerformance_logs_parallel/{key}_ram_usage_normal.csv"  # Die bestehende CSV-Datei
        csv_file_2 = f'results/singlePerformance_logs_parallel/{key}_ram_usage_cleaned.csv'
        output_file = f"results/{key}_ram_usage_parallel.csv"
    elif folder_key == "preprocessingPerformance":
        txt_file = f"results/prepoPerformance_logs/merkle_ram_usage.txt"  # Die .txt-Datei
        csv_file = f"results/prepoPerformance_logs_parallelized/merkle_ram_usage_normal.csv"  # Die bestehende CSV-Datei
        csv_file_2 = f'results/prepoPerformance_logs_parallelized/merkle_ram_usage_cleaned.csv'
        output_file = f"results/merkle_ram_usage_parallel.csv"


    with open(txt_file, "r") as txt, open(csv_file, "w", newline="") as csv_out:
        writer = csv.writer(csv_out)

        for line in txt:
            # Schreibe jede Zeile aus der TXT-Datei unverändert in die CSV-Datei
            writer.writerow([line.strip()])

    # Daten für beide Dateien bereinigen
    cleaned_data1 = []
    cleaned_data2 = []

    # Datei 1 einlesen und bereinigen
    with open(csv_file, "r") as file1:
        reader1 = csv.reader(file1)
        next(reader1, None)  # Überspringe die Header-Zeile, falls vorhanden
        for line in file1:
            match = re.match(r"^(?:Start|End) of (?:repition|repetition|repitition) (\d+): RAM Usage: (\d+) MB", line.strip())
            if match:
                repetition = int(match.group(1))
                ram_usage = int(match.group(2))
                cleaned_data1.append([repetition, ram_usage])

    # Datei 2 einlesen und bereinigen
    with open(csv_file_2, "r") as file2:
        reader2 = csv.reader(file2)
        next(reader2, None)  # Überspringe die Header-Zeile, falls vorhanden
        for line in file2:
            match = re.match(r"^(?:Start|End) of (?:repition|repetition|repitition) (\d+): RAM Usage: (\d+) MB", line.strip())
            if match:
                repetition = int(match.group(1))
                ram_usage = int(match.group(2))
                cleaned_data2.append([repetition, ram_usage])

    # Überprüfen, ob die Anzahl der Zeilen übereinstimmt
    if len(cleaned_data1) != len(cleaned_data2):
        print(len(cleaned_data1))
        print(len(cleaned_data2))
        raise ValueError("Die beiden Dateien haben unterschiedliche Mengen an Repetitions.")

    # Daten kombinieren
    merged_data = []
    for row1, row2 in zip(cleaned_data1, cleaned_data2):
        merged_data.append([row1[0], row1[1], row2[1]])

    # Ergebnisse in eine neue CSV-Datei schreiben
    with open(output_file, "w", newline="") as outfile:
        writer = csv.writer(outfile)
        writer.writerow(["Repetition", "RAM_Usage_File1_MB", "RAM_Usage_File2_MB"])
        writer.writerows(merged_data)

    print(f"Die bereinigten und kombinierten Daten wurden in {output_file} gespeichert.")



def clean_Exec_data(key, key_folder):
    file1 = None
    file2 = None
    output_file = None
    if key_folder == "singlePerformance":
        file1 = f'results/singlePerformance_logs/{key}_execution_time.txt'
        file2 = f'results/singlePerformance_logs_parallel/{key}_execution_time.txt'
        output_file = f'results/{key}_execution_time.csv'
    elif key_folder == "preprocessingPerformance":
        file1 = f'results/prepoPerformance_logs/generateMerkleProofs_execution_time.txt'
        file2 = f'results/prepoPerformance_logs_parallelized/generateMerkleProofsParallel_execution_time.txt'
        output_file = f'results/merkle_execution_time.csv'
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
    csv_file_path = 'results/CPU_usage_elliptic.csv'
    repetitions = []
    max_cpu_usage = []
    second_max_cpu_usage = []

    # Daten aus der CSV-Datei einlesen
    with open(csv_file_path, 'r') as csvfile:
        reader = csv.DictReader(csvfile)
        for row in reader:
            repetitions.append(int(row['Repetition']))
            max_cpu_usage.append(float(row['Max_CPU_Usage_us']))
            second_max_cpu_usage.append(float(row['Second_Max_CPU_Usage_us']))

    # Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, max_cpu_usage, label='CPU Usage (us)', marker='o')
    plt.scatter(repetitions, second_max_cpu_usage, label='CPU Usage (us) parallelized', marker='x')

    # Plot anpassen
    plt.title('CPU Usage Elliptic curves Comparison')
    plt.xlabel('Repetition')
    plt.ylabel('CPU Usage (us)')
    plt.legend()
    plt.grid(True)

    # Plot anzeigen
    plt.tight_layout()
    plt.savefig('figures/CPU_elliptic.png', dpi=300)


def plot_RAM_elliptic():
    csv_file = f'results/elliptic_ram_usage_parallel.csv'

    # Daten aus der CSV-Datei lesen
    repetitions = []
    ram_usage_file1 = []
    ram_usage_file2 = []

    with open(csv_file, "r") as file:
        reader = csv.reader(file)
        next(reader)  # Überspringe die Kopfzeile
        for row in reader:
            repetitions.append(int(row[0]))
            ram_usage_file1.append(int(row[1]))
            ram_usage_file2.append(int(row[2]))

    # Scatter-Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, ram_usage_file1, color="blue", label="RAM Usage (MB)", marker="o")
    plt.scatter(repetitions, ram_usage_file2, color="red", label="RAM Usage (MB) Parallelized", marker="o")

    # Achsenbeschriftungen und Titel
    plt.xlabel("Repetition")
    plt.ylabel("RAM Usage (MB)")
    plt.title("RAM Usage per Repetition")
    plt.legend()

    # Diagramm anzeigen
    plt.grid(True)
    plt.savefig('figures/RAM_elliptic.png', dpi=300)


def plot_Exec_elliptic():
    # Pfad zur CSV-Datei
    csv_file = f'results/elliptic_execution_time.csv'

    # Daten aus der CSV-Datei lesen
    repetitions = []
    execution_time_1 = []
    execution_time_2 = []

    with open(csv_file, "r") as file:
        reader = csv.reader(file)
        next(reader)  # Überspringe die Kopfzeile
        for row in reader:
            repetitions.append(int(row[0]))
            execution_time_1.append(float(row[1]))
            execution_time_2.append(float(row[2]))

    # Scatter-Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, execution_time_1, color="blue", label="Execution_Time", marker="o")
    plt.scatter(repetitions, execution_time_2, color="red", label="Execution_Time_Parallelized", marker="o")

    # Achsenbeschriftungen und Titel
    plt.xlabel("Repetition")
    plt.ylabel("Execution Time (ms)")
    plt.title("Execution Time per Repetition for elliptic curve")
    plt.legend()

    # Diagramm anzeigen
    plt.grid(True)
    plt.savefig('figures/execution_elliptic.png', dpi=300)


def plot_CPU_LWE():
    csv_file_path = 'results/CPU_usage_lwe.csv'
    repetitions = []
    max_cpu_usage = []
    second_max_cpu_usage = []

    # Daten aus der CSV-Datei einlesen
    with open(csv_file_path, 'r') as csvfile:
        reader = csv.DictReader(csvfile)
        for row in reader:
            repetitions.append(int(row['Repetition']))
            max_cpu_usage.append(float(row['Max_CPU_Usage_us']))
            second_max_cpu_usage.append(float(row['Second_Max_CPU_Usage_us']))

    # Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, max_cpu_usage, label='CPU Usage (us)', marker='o')
    plt.scatter(repetitions, second_max_cpu_usage, label='CPU Usage (us) parallelized', marker='x')

    # Plot anpassen
    plt.title('CPU Usage of LWE Comparison')
    plt.xlabel('Repetition')
    plt.ylabel('CPU Usage (us)')
    plt.legend()
    plt.grid(True)

    # Plot anzeigen
    plt.tight_layout()
    plt.savefig('figures/CPU_LWE.png', dpi=300)


def plot_RAM_LWE():
    csv_file = f'results/lwe_ram_usage_parallel.csv'

    # Daten aus der CSV-Datei lesen
    repetitions = []
    ram_usage_file1 = []
    ram_usage_file2 = []

    with open(csv_file, "r") as file:
        reader = csv.reader(file)
        next(reader)  # Überspringe die Kopfzeile
        for row in reader:
            repetitions.append(int(row[0]))
            ram_usage_file1.append(int(row[1]))
            ram_usage_file2.append(int(row[2]))

    # Scatter-Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, ram_usage_file1, color="blue", label="RAM Usage (MB)", marker="o")
    plt.scatter(repetitions, ram_usage_file2, color="red", label="RAM Usage (MB) Parallelized", marker="o")

    # Achsenbeschriftungen und Titel
    plt.xlabel("Repetition")
    plt.ylabel("RAM Usage (MB)")
    plt.title("RAM Usage per Repetition")
    plt.legend()

    # Diagramm anzeigen
    plt.grid(True)
    plt.savefig('figures/RAM_LWE.png', dpi=300)


def plot_Exec_LWE():
    # Pfad zur CSV-Datei
    csv_file = f'results/lwe_execution_time.csv'

    # Daten aus der CSV-Datei lesen
    repetitions = []
    execution_time_1 = []
    execution_time_2 = []

    with open(csv_file, "r") as file:
        reader = csv.reader(file)
        next(reader)  # Überspringe die Kopfzeile
        for row in reader:
            repetitions.append(int(row[0]))
            execution_time_1.append(float(row[1]))
            execution_time_2.append(float(row[2]))

    # Scatter-Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, execution_time_1, color="blue", label="Execution_Time", marker="o")
    plt.scatter(repetitions, execution_time_2, color="red", label="Execution_Time_Parallelized", marker="o")

    # Achsenbeschriftungen und Titel
    plt.xlabel("Repetition")
    plt.ylabel("Execution Time (ms)")
    plt.title("Execution Time per Repetition for LWE")
    plt.legend()

    # Diagramm anzeigen
    plt.grid(True)
    plt.savefig('figures/execution_lwe.png', dpi=300)


def plot_CPU_LWE128():
    csv_file_path = 'results/CPU_usage_lwe128.csv'
    repetitions = []
    max_cpu_usage = []
    second_max_cpu_usage = []

    # Daten aus der CSV-Datei einlesen
    with open(csv_file_path, 'r') as csvfile:
        reader = csv.DictReader(csvfile)
        for row in reader:
            repetitions.append(int(row['Repetition']))
            max_cpu_usage.append(float(row['Max_CPU_Usage_us']))
            second_max_cpu_usage.append(float(row['Second_Max_CPU_Usage_us']))

    # Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, max_cpu_usage, label='CPU Usage (us)', marker='o')
    plt.scatter(repetitions, second_max_cpu_usage, label='CPU Usage (us) parallelized', marker='x')

    # Plot anpassen
    plt.title('CPU Usage Elliptic curves Comparison')
    plt.xlabel('Repetition')
    plt.ylabel('CPU Usage (us)')
    plt.legend()
    plt.grid(True)

    # Plot anzeigen
    plt.tight_layout()
    plt.savefig('figures/CPU_lwe128.png', dpi=300)


def plot_RAM_LWE128():
    csv_file = f'results/lwe128_ram_usage_parallel.csv'

    # Daten aus der CSV-Datei lesen
    repetitions = []
    ram_usage_file1 = []
    ram_usage_file2 = []

    with open(csv_file, "r") as file:
        reader = csv.reader(file)
        next(reader)  # Überspringe die Kopfzeile
        for row in reader:
            repetitions.append(int(row[0]))
            ram_usage_file1.append(int(row[1]))
            ram_usage_file2.append(int(row[2]))

    # Scatter-Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, ram_usage_file1, color="blue", label="RAM Usage (MB)", marker="o")
    plt.scatter(repetitions, ram_usage_file2, color="red", label="RAM Usage (MB) Parallelized", marker="o")

    # Achsenbeschriftungen und Titel
    plt.xlabel("Repetition")
    plt.ylabel("RAM Usage (MB)")
    plt.title("RAM Usage per Repetition")
    plt.legend()

    # Diagramm anzeigen
    plt.grid(True)
    plt.savefig('figures/ram_lwe128.png', dpi=300)


def plot_Exec_LWE128():
    # Pfad zur CSV-Datei
    csv_file = f'results/lwe128_execution_time.csv'

    # Daten aus der CSV-Datei lesen
    repetitions = []
    execution_time_1 = []
    execution_time_2 = []

    with open(csv_file, "r") as file:
        reader = csv.reader(file)
        next(reader)  # Überspringe die Kopfzeile
        for row in reader:
            repetitions.append(int(row[0]))
            execution_time_1.append(float(row[1]))
            execution_time_2.append(float(row[2]))

    # Scatter-Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, execution_time_1, color="blue", label="Execution_Time", marker="o")
    plt.scatter(repetitions, execution_time_2, color="red", label="Execution_Time_Parallelized", marker="o")

    # Achsenbeschriftungen und Titel
    plt.xlabel("Repetition")
    plt.ylabel("Execution Time (ms)")
    plt.title("Execution Time per Repetition LWE128")
    plt.legend()

    # Diagramm anzeigen
    plt.grid(True)
    plt.savefig('figures/execution_lwe128.png', dpi=300)


def plot_CPU_merkle():
    csv_file_path = 'results/CPU_usage_merkle.csv'
    repetitions = []
    max_cpu_usage = []
    second_max_cpu_usage = []

    # Daten aus der CSV-Datei einlesen
    with open(csv_file_path, 'r') as csvfile:
        reader = csv.DictReader(csvfile)
        for row in reader:
            repetitions.append(int(row['Repetition']))
            max_cpu_usage.append(float(row['Max_CPU_Usage_us']))
            second_max_cpu_usage.append(float(row['Second_Max_CPU_Usage_us']))

    # Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, max_cpu_usage, label='CPU Usage (us)', marker='o')
    plt.scatter(repetitions, second_max_cpu_usage, label='CPU Usage (us) parallelized', marker='x')

    # Plot anpassen
    plt.title('CPU Usage Elliptic curves Comparison')
    plt.xlabel('Repetition')
    plt.ylabel('CPU Usage (us)')
    plt.legend()
    plt.grid(True)

    # Plot anzeigen
    plt.tight_layout()
    plt.savefig('figures/CPU_merkle.png', dpi=300)


def plot_RAM_merkle():
    csv_file = f'results/merkle_ram_usage_parallel.csv'

    # Daten aus der CSV-Datei lesen
    repetitions = []
    ram_usage_file1 = []
    ram_usage_file2 = []

    with open(csv_file, "r") as file:
        reader = csv.reader(file)
        next(reader)  # Überspringe die Kopfzeile
        for row in reader:
            repetitions.append(int(row[0]))
            ram_usage_file1.append(int(row[1]))
            ram_usage_file2.append(int(row[2]))

    # Scatter-Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, ram_usage_file1, color="blue", label="RAM Usage (MB)", marker="o")
    plt.scatter(repetitions, ram_usage_file2, color="red", label="RAM Usage (MB) Parallelized", marker="o")

    # Achsenbeschriftungen und Titel
    plt.xlabel("Repetition")
    plt.ylabel("RAM Usage (MB)")
    plt.title("RAM Usage per Repetition")
    plt.legend()

    # Diagramm anzeigen
    plt.grid(True)
    plt.savefig('figures/ram_merkle.png', dpi=300)


def plot_Exec_merkle():
    # Pfad zur CSV-Datei
    csv_file = f'results/merkle_execution_time.csv'

    # Daten aus der CSV-Datei lesen
    repetitions = []
    execution_time_1 = []
    execution_time_2 = []

    with open(csv_file, "r") as file:
        reader = csv.reader(file)
        next(reader)  # Überspringe die Kopfzeile
        for row in reader:
            repetitions.append(int(row[0]))
            execution_time_1.append(float(row[1]))
            execution_time_2.append(float(row[2]))

    # Scatter-Plot erstellen
    plt.figure(figsize=(10, 6))
    plt.scatter(repetitions, execution_time_1, color="blue", label="Execution_Time", marker="o")
    plt.scatter(repetitions, execution_time_2, color="red", label="Execution_Time_Parallelized", marker="o")

    # Achsenbeschriftungen und Titel
    plt.xlabel("Repetition")
    plt.ylabel("Execution Time (ms)")
    plt.title("Execution Time per Repetition for merkle tree")
    plt.legend()

    # Diagramm anzeigen
    plt.grid(True)
    plt.savefig('figures/execution_merkle.png', dpi=300)


if __name__ == '__main__':
    plot_RAM_merkle()
    plot_RAM_LWE128()
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

    if folder_key == "singlePerformance":
        list = {"lwe128", "lwe", "elliptic"}
        for key in list:
            #clean_CPU_data(key, True, folder_key)
            sort_CPU_usage(key, folder_key)
            clean_CPU_data(key, False, folder_key)
            merge_CPU_Usage(key, folder_key)
            clean_Exec_data(key, folder_key)
            sort_ram_usage(key, folder_key)
            clean_RAM_data(key, folder_key)

            plot_Exec_elliptic()
            plot_CPU_elliptic()
            plot_RAM_elliptic()

            plot_Exec_LWE()
            plot_CPU_LWE()
            plot_RAM_LWE()

            plot_Exec_LWE128()
            plot_CPU_LWE128()
            plot_RAM_LWE128()
    elif folder_key == "preprocessingPerformance":
            clean_CPU_data(None, True, folder_key)
            sort_CPU_usage(None, folder_key)
            clean_CPU_data(None, False, folder_key)
            merge_CPU_Usage(None, folder_key)
            clean_Exec_data(None, folder_key)
            sort_ram_usage(None, folder_key)
            clean_RAM_data(None, folder_key)
            plot_Exec_merkle()
            plot_CPU_merkle()
            plot_RAM_merkle()



