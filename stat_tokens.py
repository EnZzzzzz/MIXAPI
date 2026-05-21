#!/usr/bin/env python3
"""统计指定月份的总输入token数、总输出token数、总请求次数。"""

import pymysql
import argparse
from datetime import datetime


def main():
    parser = argparse.ArgumentParser(description="统计指定月份的 token 使用量")
    parser.add_argument("month", help="月份，格式：YYYY-MM，例如 2026-05")
    args = parser.parse_args()

    # 解析月份范围
    try:
        start = datetime.strptime(args.month, "%Y-%m")
    except ValueError:
        print("错误：月份格式必须为 YYYY-MM，例如 2026-05")
        return

    # 下个月初作为结束时间
    if start.month == 12:
        end = start.replace(year=start.year + 1, month=1)
    else:
        end = start.replace(month=start.month + 1)

    start_ts = int(start.timestamp())
    end_ts = int(end.timestamp())

    conn = pymysql.connect(
        host="43.139.21.243",
        port=3306,
        user="root",
        password="Ucd2024.",
        database="mixapi",
        charset="utf8mb4",
    )

    try:
        with conn.cursor() as cur:
            sql = """
                SELECT
                    COALESCE(SUM(prompt_tokens), 0),
                    COALESCE(SUM(completion_tokens), 0),
                    COUNT(*)
                FROM logs
                WHERE created_at >= %s AND created_at < %s
            """
            cur.execute(sql, (start_ts, end_ts))
            row = cur.fetchone()

        total_input, total_output, total_reqs = row
        month_label = args.month

        print(f"=== {month_label} 统计 ===")
        print(f"总输入 Token（prompt_tokens）    : {total_input:>12,}")
        print(f"总输出 Token（completion_tokens） : {total_output:>12,}")
        print(f"总请求次数                       : {total_reqs:>12,}")

        total_tokens = total_input + total_output
        if total_tokens > 0:
            ratio = total_output / total_tokens * 100
            print(f"\n输出占比: {ratio:.1f}%")

    finally:
        conn.close()


if __name__ == "__main__":
    main()
