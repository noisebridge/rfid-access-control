/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 *
 * Debug output: dumping all RC522 registers.
 */
#ifndef _MFRC522_DEBUG_H
#define _MFRC522_DEBUG_H
class SerialCom;
class MFRC522;
void showRFIDStatus(SerialCom *out, MFRC522 *reader);
#endif  // _MFRC522_DEBUG_H
