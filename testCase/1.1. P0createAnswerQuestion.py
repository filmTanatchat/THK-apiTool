import os
import pandas as pd
import json
import base64
import csv
import re
from datetime import datetime


def convert_date_to_unix(date_str, is_datetime=False):
    try:
        date_str = str(date_str)
        if is_datetime:
            dt = datetime.strptime(date_str, "%d-%m-%Y %H:%M:%S")
        else:
            dt = datetime.strptime(date_str, "%d-%m-%Y")
        return int(dt.timestamp())
    except ValueError:
        return date_str


def get_file_type(file_path):
    extension_to_type = {
        ".jpg": "jpeg",
        ".jpeg": "jpeg",
        ".png": "png",
        ".pdf": "pdf",
        # add more file types as needed
    }
    _, ext = os.path.splitext(file_path)
    return extension_to_type.get(ext, "unknown")


def compress_and_encode(file_path):
    try:
        file_type = get_file_type(file_path)
        if file_type == "unknown":
            print(f"Warning: Unknown file type for {file_path}")
            return ""

        with open(file_path, "rb") as f:
            encoded_data = base64.b64encode(f.read()).decode()
            return f"data:@file/{file_type};base64,{encoded_data}"
    except FileNotFoundError:
        print(f"File not found: {file_path}")
        return ""


def process_and_save_csv(file_path):
    df = pd.read_csv(file_path, dtype=str)

    for col in df.columns:
        if col == "case_id":
            continue

        field_name, data_type, *rest = col.split("||")
        is_multi = "MULTI" in rest

        if data_type == "date":
            df[col] = df[col].apply(
                lambda x: convert_date_to_unix(x) if pd.notna(x) else x
            )
        elif data_type == "file":
            process_file_column(df, col, is_multi)
        elif is_multi:
            # Format multi-value fields without extra backslashes
            df[col] = df[col].apply(
                lambda x: "[" + ", ".join(f'"{val}"' for val in x.split(",")) + "]"
                if pd.notna(x) and x != ""
                else "[]"
            )

        df.rename(columns={col: field_name}, inplace=True)

    new_file_path = file_path.replace("question", "answer")
    df.to_csv(
        new_file_path,
        index=False,
        escapechar="\\",
        doublequote=True,
        quoting=csv.QUOTE_NONNUMERIC,
    )

    return len(df), len(df.columns), new_file_path


def process_file_column(df, col, is_multi):
    if is_multi:
        df[col] = df[col].apply(
            lambda x: json.dumps(
                [
                    compress_and_encode(
                        os.path.join(BASE_PATH, "4. answerAndQuestion", "file", f)
                    )
                    for f in str(x).split("\\")
                ]
            )
            if pd.notna(x)
            else "[]"
        )
    else:
        df[col] = df[col].apply(
            lambda x: compress_and_encode(
                os.path.join(BASE_PATH, "4. answerAndQuestion", "file", x)
            )
            if pd.notna(x)
            else ""
        )


def save_as_json(new_df, json_path):
    json_data = []
    for _, row in new_df.iterrows():
        entry = {
            "case_id": str(row["case_id"]),
            "is_question_mode": False,
            "answers": [
                {
                    "field_name": col,
                    "field_value": str(row[col])
                    if pd.notna(row[col]) and row[col] != "[]"
                    else "",
                    "source": "customer",
                }
                for col in new_df.columns
                if col != "case_id"
            ],
        }
        json_data.append(entry)

    with open(json_path, "w", encoding="utf-8") as f:
        json.dump(json_data, f, ensure_ascii=False, indent=4)


def main():
    print(f"Attempting to read from: {CSV_PATH}")
    if not os.path.exists(CSV_PATH):
        print(f"Error: The file {CSV_PATH} does not exist.")
        return

    num_rows, num_processed_cols, csv_output = process_and_save_csv(CSV_PATH)

    print("\n--- Summary ---")
    print(f"Processed {num_rows} rows.")
    print(f"Processed columns: {num_processed_cols}")
    print(f"Output CSV file: {csv_output}")

    # If you need the JSON output path, construct it here
    json_output = csv_output.replace(".csv", ".json")
    print(f"Output JSON file: {json_output}")


if __name__ == "__main__":
    current_script_path = os.path.dirname(os.path.realpath(__file__))
    BASE_PATH = os.path.dirname(current_script_path)
    CSV_PATH = os.path.join(BASE_PATH, "4. answerAndQuestion", "P0questionTestCase.csv")
    main()
