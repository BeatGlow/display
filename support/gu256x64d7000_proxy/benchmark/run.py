"""Run the Nano Every frame benchmark under MPLAB X MDB."""

import argparse
import os
from pathlib import Path
import random
import re
import subprocess


def fixture(path):
    rng = random.Random(26)
    mono = [rng.randrange(2) for _ in range(256 * 64)]
    gray3 = [rng.randrange(3) for _ in mono]
    gray2 = [rng.randrange(4) for _ in mono]

    def pack(pixels, group, base):
        return [
            sum(value * base**digit for digit, value in enumerate(pixels[i:i + group]))
            for i in range(0, len(pixels), group)
        ]

    frames = {
        "monoFixture": [
            sum(value << (7 - bit) for bit, value in enumerate(mono[i:i + 8]))
            for i in range(0, len(mono), 8)
        ],
        "gray3Fixture": [
            byte
            for i in range(0, len(gray3), 64)
            for byte in pack(gray3[i:i + 64], 5, 3)
        ],
        "gray2Fixture": pack(gray2, 4, 4),
    }
    with path.open("w") as output:
        for name, values in frames.items():
            print(f"static const uint8_t {name}[] PROGMEM = {{", file=output)
            for i in range(0, len(values), 24):
                print("  " + ", ".join(map(str, values[i:i + 24])) + ",", file=output)
            print("};", file=output)


def mdb_commands(elf):
    commands = [
        "Device ATmega4809", "Set oscillator.frequency 16", "Hwtool SIM",
        f'Program "{elf}"',
        "Break benchStart", "Break benchCRCComplete", "Break benchPublished", "Break benchEnd",
        "Run", "Wait 10000",
    ]
    for i in range(3):
        if i:
            commands.extend(["Continue", "Wait 10000"])
        for _ in range(3):
            commands.extend(["Stopwatch clear", "Continue", "Wait 10000", "Stopwatch"])
        commands.extend(["Print benchmarkOp", "Print benchmarkOK"])
    return "\n".join(commands + ["Quit", ""])


def print_results(log):
    elapsed = [
        int(cycles) for cycles in re.findall(
            r"^Stopwatch cycle count = (\d+) ", log, flags=re.MULTILINE
        )
    ]
    opcodes = re.findall(r"benchmarkOp=\s*(\d+)", log)
    passed = re.findall(r"benchmarkOK=\s*(\d+)", log)
    if len(elapsed) != 9 or opcodes != ["2", "3", "4"] or passed != ["1"] * 3:
        raise SystemExit("Expected three passing simulator trials. Inspect mdb.log.")
    print("Mode       Bytes   CRC cyc  Request cyc  Next mask cyc  SPI ms  USB ms")
    for i, (name, size) in enumerate([("Mono", 2048), ("Gray3Level", 3328), ("Gray2", 4096)]):
        crc, request, next_mask = elapsed[3 * i:3 * i + 3]
        cpu_ms = (crc + request) / 16000
        spi_ms = cpu_ms + size * 0.004
        usb_ms = cpu_ms + size * 0.010
        print(f"{name:10} {size:5} {crc:9} {request:12} {next_mask:14} {spi_ms:7.2f} {usb_ms:7.2f}")
    print("Totals combine payload wire time and cycles through frame acknowledgment work.")
    print("PBUSY is held ready. Handshake, AVR interrupts and host scheduling are excluded.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mdb", required=True, type=Path)
    parser.add_argument("--avr-cxx", required=True, type=Path)
    parser.add_argument("--work-dir", required=True, type=Path)
    args = parser.parse_args()

    work = args.work_dir.resolve()
    work.mkdir(parents=True, exist_ok=True)
    source = Path(__file__).resolve().parent
    fixture(work / "fixture.h")
    elf = work / "bench.elf"
    env = os.environ.copy()
    subprocess.run([
        str(args.avr_cxx.resolve()), "-mmcu=atmega4809", "-Os", "-g", "-std=gnu++11",
        "-DARDUINO_AVR_NANO_EVERY", "-I", str(source), "-I", str(work),
        str(source / "bench.cpp"), "-o", str(elf),
    ], cwd=work, env=env, check=True)
    commands = work / "bench.mdb"
    commands.write_text(mdb_commands(elf))
    env["USER_DIR"] = str(work / "mplab-user")
    env["JAVA_TOOL_OPTIONS"] = f'-Djava.io.tmpdir="{work}" -Duser.home="{work}"'
    with (work / "mdb.log").open("w") as log:
        subprocess.run(
            [str(args.mdb.resolve()), str(commands)], cwd=work, env=env,
            stdout=log, stderr=subprocess.STDOUT, timeout=180, check=True,
        )
    print_results((work / "mdb.log").read_text(errors="replace"))


if __name__ == "__main__":
    main()
