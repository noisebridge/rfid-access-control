/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */
#include "mfrc522-debug.h"
#include "mfrc522/mfrc522.h"
#include "serial-com.h"

static MFRC522::PCD_Register all_reg[] = {
  MFRC522::CommandReg,
  MFRC522::ComIEnReg,
  MFRC522::DivIEnReg,
  MFRC522::ComIrqReg,
  MFRC522::DivIrqReg,
  MFRC522::ErrorReg,
  MFRC522::Status1Reg,
  MFRC522::Status2Reg,
  MFRC522::FIFODataReg,
  MFRC522::FIFOLevelReg,
  MFRC522::WaterLevelReg,
  MFRC522::ControlReg,
  MFRC522::BitFramingReg,
  MFRC522::CollReg,

  MFRC522::ModeReg,
  MFRC522::TxModeReg,
  MFRC522::RxModeReg,
  MFRC522::TxControlReg,
  MFRC522::TxASKReg,
  MFRC522::TxSelReg,
  MFRC522::RxSelReg,
  MFRC522::RxThresholdReg,
  MFRC522::DemodReg,
  MFRC522::MfTxReg,
  MFRC522::MfRxReg,
  MFRC522::SerialSpeedReg,

  MFRC522::CRCResultRegH,
  MFRC522::CRCResultRegL,
  MFRC522::ModWidthReg,
  MFRC522::RFCfgReg,
  MFRC522::GsNReg,
  MFRC522::CWGsPReg,
  MFRC522::ModGsPReg,
  MFRC522::TModeReg,
  MFRC522::TPrescalerReg,
  MFRC522::TReloadRegH,
  MFRC522::TReloadRegL,
  MFRC522::TCounterValueRegH,
  MFRC522::TCounterValueRegL,

  MFRC522::TestSel1Reg,
  MFRC522::TestSel2Reg,
  MFRC522::TestPinEnReg,
  MFRC522::TestPinValueReg,
  MFRC522::TestBusReg,
  MFRC522::AutoTestReg,
  MFRC522::VersionReg,
  MFRC522::AnalogTestReg,
  MFRC522::TestDAC1Reg,
  MFRC522::TestDAC2Reg,
  MFRC522::TestADCReg,
};

// TODO: this code is duplicate. But we don't worry too much as this typically
// doesn't make it into 'production' code.
static char to_hex(unsigned char c) { return c < 0x0a ? c + '0' : c + 'a' - 10; }
static void printHexByte(SerialCom *out, unsigned char c) {
  out->write(to_hex(c >> 4));
  out->write(to_hex(c & 0x0f));
}

void showRFIDStatus(SerialCom *out, MFRC522 *reader) {
  for (byte i = 0; i < sizeof(all_reg)/sizeof(MFRC522::PCD_Register); ++i) {
    out->write('#');
    printHexByte(out, all_reg[i]);
    out->write(' ');
    printHexByte(out, reader->PCD_ReadRegister(all_reg[i]));
    out->write('\r');
    out->write('\n');
  }
}
