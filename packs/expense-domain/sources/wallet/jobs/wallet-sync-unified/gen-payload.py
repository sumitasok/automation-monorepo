import json
CC="97320818-c6df-4fbc-be24-baa5fbea7cc5"
SB="6cf80ab9-85bd-420a-aec4-8498005f4ce8"
REST="5c5c03e9-000a-8000-8000-000000000000"
TAXI="5c5c0fa1-0028-8000-8000-000000000000"
SHOP="5c5c07da-0014-8000-8000-000000000000"
CLOTH="5c5c07d0-0014-8000-8000-000000000000"
SUBS="5c5c1777-003c-8000-8000-000000000000"
UTIL="5c5c0bba-001e-8000-8000-000000000000"
TAX="5c5c1f40-0050-8000-8000-000000000000"
L={"amazon":"6a6cb3bc-cac2-407c-b700-494a916e2da9","online":"4b764bbb-eecc-4031-bb04-4a55c9467ee9",
"blr4work":"6fc217ef-040c-43ec-9bab-543af65dd8da","incity":"c698e267-34b4-4376-bb28-ec4e3f16fd84",
"chinju":"56226bbd-5d97-435a-af7e-18e7b6cf348f","subscription":"d52290f6-170f-45f5-86c4-d1eb0e57cd74",
"apple":"4ef4f75b-2d95-4e09-8df8-336ece76f7df","kseb":"dcf5b342-f9a2-4666-9cda-b6c18e98eca0"}

def cc(date,amt,cp,cat,gm,labels,inst="HDFC CC x3690"):
    return {"accountId":CC,"amount":amt,"recordDate":date,"paymentType":"credit_card",
            "categoryId":cat,"counterParty":cp,"note":cp+" | via "+inst+" | gmail-sync gm:"+gm,"labelIds":labels}
def sb(date,amt,cp,cat,gm,labels):
    r={"accountId":SB,"amount":amt,"recordDate":date,"paymentType":"mobile_payment",
       "counterParty":cp,"note":cp+" | via HDFC SB x3176 UPI | gmail-sync gm:"+gm,"labelIds":labels}
    if cat: r["categoryId"]=cat
    return r

hb=[L["blr4work"]]
uber=[L["incity"]]
amz=[L["amazon"],L["online"]]
apple=[L["subscription"],L["apple"]]

cc_recs=[
 cc("2026-07-22",-14,"HungerBox",REST,"19f886ca18c21983",hb),
 cc("2026-07-23",-47,"HungerBox",REST,"19f8cf2e8103e19d",hb),
 cc("2026-07-23",-40,"HungerBox",REST,"19f8d0e998a51ad0",hb),
 cc("2026-07-23",-779,"RAMESHWARI (cloth for Chinju)",CLOTH,"19f8d5720e3b6144",[L["chinju"]]),
 cc("2026-07-23",-137,"HungerBox",REST,"19f8debcfcf57abe",hb),
 cc("2026-07-23",-193.56,"Uber",TAXI,"19f8e29530880948",uber),
 cc("2026-07-24",-47,"HungerBox",REST,"19f9214042276ee9",hb),
 cc("2026-07-24",-40,"HungerBox",REST,"19f9232ac3e849c6",hb),
 cc("2026-07-24",-32,"HungerBox",REST,"19f9291a281d54a9",hb),
 cc("2026-07-24",-137,"HungerBox",REST,"19f931b115a2d76f",hb),
 cc("2026-07-24",-40,"HungerBox",REST,"19f933a27f6d0d0f",hb),
 cc("2026-07-26",-3038.27,"ASSPL (Amazon)",SHOP,"19f9f3ff4d1b16f2",amz),
 cc("2026-07-27",-173.07,"Uber",TAXI,"19fa2d0cbf7f1c13",uber),
 cc("2026-07-30",-197.26,"Uber",TAXI,"19fb15f5b363bc7a",uber),
 cc("2026-07-31",-163.27,"Uber",TAXI,"19fb6f1dc8b32f37",uber),
 cc("2026-07-31",-40,"HungerBox",REST,"19fb751a1368ef6c",hb),
 cc("2026-08-03",-235,"HungerBox",REST,"19fc695cc5caf94c",hb),
 cc("2026-08-03",-40,"Eat Good Technologies",REST,"19fc6b36459eacef",hb),
 cc("2026-08-04",-150,"HungerBox",REST,"19fcbb7a5b870219",hb),
 cc("2026-08-04",-40,"HungerBox",REST,"19fcbe2d196d04b3",hb),
 cc("2026-08-05",-237.09,"Uber",TAXI,"19fd05cb9c01ca2c",uber),
 cc("2026-08-06",-258.43,"Uber",TAXI,"19fd5a2f9346082c",uber),
 cc("2026-08-07",-224.05,"Uber",TAXI,"19fdaa7d48d4a167",uber),
 cc("2026-08-10",-1254.64,"ASSPL (Amazon)",SHOP,"19fec5da74de80f6",amz),
 cc("2026-08-11",-190.02,"Uber",TAXI,"19fef5452044b07e",uber),
 cc("2026-08-11",190.02,"Uber (refund)",TAXI,"19fef7ce82467fba",uber),
 cc("2026-08-12",-3758.19,"ASSPL (Amazon)",SHOP,"19ff4996d9de7e11",amz),
 cc("2026-08-15",-1450.22,"ASSPL (Amazon)",SHOP,"1a0019c29e29b5d0",amz),
]
sb_recs=[
 sb("2026-07-24",-148,"Google India Digital Services (utility)",UTIL,"19f93994ffbd8e55",[L["kseb"]]),
 sb("2026-08-01",-365,"Apple Media Services",SUBS,"19fbe07a76d55d34",apple),
 sb("2026-08-02",-612,"Google India Digital Services (utility)",UTIL,"19fc0cecb2af134c",[L["kseb"]]),
 sb("2026-08-03",-253,"Ajith Kumar A",None,"19fc6121e475f553",[]),
 sb("2026-08-04",-261,"Omkar Sharma",None,"19fcb3ac55a08719",[]),
 sb("2026-08-04",-170,"Apple Services",SUBS,"19fcc9b0262b7aca",apple),
 sb("2026-08-05",-1500,"Mr Arun KR",None,"19fd228337bf18e9",[]),
 sb("2026-08-08",-1999,"Apple Media Services",SUBS,"19fe0fc7b84ed2f3",apple),
 sb("2026-08-12",-280,"Ashoka AR",None,"19ff45a45244b6c3",[]),
 sb("2026-08-12",-1650,"Karnataka Dept of Treasuries",TAX,"19ff6568a6b20048",[]),
 sb("2026-08-13",-214,"RKIT",None,"19ff99c0ee004c78",[]),
 sb("2026-08-14",-284,"Srinivas",None,"19ffead969aa659b",[]),
]
p="/Users/sumitasok/Library/Mobile Documents/iCloud~md~obsidian/Documents/sa.finances/_db/wallet-sync/_run_payload.json"
json.dump({"cc1":cc_recs[:14],"cc2":cc_recs[14:],"sb":sb_recs},open(p,"w"))
print("counts cc",len(cc_recs),"sb",len(sb_recs))
PYEOF_MARKER = None
