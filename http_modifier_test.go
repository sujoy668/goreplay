package main

import (
	"bytes"
	"testing"

	"github.com/buger/goreplay/proto"
)

func TestHTTPModifierWithoutConfig(t *testing.T) {
	if NewHTTPModifier(&HTTPModifierConfig{}) != nil {
		t.Error("If no config specified should not be initialized")
	}
}

func TestHTTPModifierHeaderFilters(t *testing.T) {
	filters := HTTPHeaderFilters{}
	filters.Set("Host:^www.w3.org$")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		HeaderFilters: filters,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if len(modifier.Rewrite(payload)) == 0 {
		t.Error("Request should pass filters")
	}

	filters = HTTPHeaderFilters{}
	// Setting filter that not match our header
	filters.Set("Host:^www.w4.org$")

	modifier = NewHTTPModifier(&HTTPModifierConfig{
		HeaderFilters: filters,
	})

	if len(modifier.Rewrite(payload)) != 0 {
		t.Error("Request should not pass filters")
	}
}

func TestHTTPModifierHeaderNegativeFilters(t *testing.T) {
	filters := HTTPHeaderFilters{}
	filters.Set("Host:^www.w3.org$")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		HeaderNegativeFilters: filters,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w4.org\r\n\r\na=1&b=2")

	if len(modifier.Rewrite(payload)) == 0 {
		t.Error("Request should pass filters")
	}

	filters = HTTPHeaderFilters{}
	// Setting filter that not match our header
	filters.Set("Host:^www.w4.org$")

	modifier = NewHTTPModifier(&HTTPModifierConfig{
		HeaderNegativeFilters: filters,
	})

	if len(modifier.Rewrite(payload)) != 0 {
		t.Error("Request should not pass filters")
	}

	filters = HTTPHeaderFilters{}
	// Setting filter that not match our header
	filters.Set("Host: www*")

	modifier = NewHTTPModifier(&HTTPModifierConfig{
		HeaderNegativeFilters: filters,
	})

	if len(modifier.Rewrite(payload)) != 0 {
		t.Error("Request should not pass filters")
	}
}

func TestHTTPHeaderBasicAuthFilters(t *testing.T) {
	filters := HTTPHeaderBasicAuthFilters{}
	filters.Set("^customer[0-9].*")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		HeaderBasicAuthFilters: filters,
	})

	//Encoded UserId:Password = customer3:welcome
	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nAuthorization: Basic Y3VzdG9tZXIzOndlbGNvbWU=\r\n\r\na=1&b=2")
	if len(modifier.Rewrite(payload)) == 0 {
		t.Error("Request should pass filters")
	}

	//customer6:rest@123^TEST
	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 88\r\nAuthorization: Basic Y3VzdG9tZXI2OnJlc3RAMTIzXlRFU1Q==\r\n\r\na=1&b=2")
	if len(modifier.Rewrite(payload)) == 0 {
		t.Error("Request should pass filters")
	}

	filters = HTTPHeaderBasicAuthFilters{}
	// Setting filter that not match our header
	filters.Set("^(homer simpson|mickey mouse).*")

	modifier = NewHTTPModifier(&HTTPModifierConfig{
		HeaderBasicAuthFilters: filters,
	})

	if len(modifier.Rewrite(payload)) != 0 {
		t.Error("Request should not pass filters")
	}

	//mickey mouse:happy123
	payload = []byte("POST /post HTTP/1.1\r\nContent-Length: 88\r\nAuthorization: Basic bWlja2V5IG1vdXNlOmhhcHB5MTIz\r\n\r\na=1&b=2")
	if len(modifier.Rewrite(payload)) == 0 {
		t.Error("Request should pass filters")
	}
}

func TestHTTPModifierURLRewrite(t *testing.T) {
	var url, newURL []byte

	rewrites := URLRewriteMap{}

	payload := func(url []byte) []byte {
		return []byte("POST " + string(url) + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	err := rewrites.Set("/v1/user/([^\\/]+)/ping:/v2/user/$1/ping")
	if err != nil {
		t.Error("Should not error on /v1/user/([^\\/]+)/ping:/v2/user/$1/ping")
	}

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		URLRewrite: rewrites,
	})

	url = []byte("/v1/user/joe/ping")
	if newURL = proto.Path(modifier.Rewrite(payload(url))); bytes.Equal(newURL, url) {
		t.Error("Request url should have been rewritten, wasn't", string(newURL))
	}

	url = []byte("/v1/user/ping")
	if newURL = proto.Path(modifier.Rewrite(payload(url))); !bytes.Equal(newURL, url) {
		t.Error("Request url should have been rewritten, wasn't", string(newURL))
	}
}

func TestHTTPModifierHeaderRewrite(t *testing.T) {
	var header, newHeader []byte

	rewrites := HeaderRewriteMap{}
	payload := []byte("GET / HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	err := rewrites.Set("Host: (.*).w3.org,$1.beta.w3.org")
	if err != nil {
		t.Error("Should not error", err)
	}

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		HeaderRewrite: rewrites,
	})

	header = []byte("www.beta.w3.org")
	if newHeader = proto.Header(modifier.Rewrite(payload), []byte("Host")); !bytes.Equal(newHeader, header) {
		t.Error("Request header should have been rewritten, wasn't", string(newHeader), string(header))
	}
}

func TestHTTPModifierHeaderHashFilters(t *testing.T) {
	filters := HTTPHashFilters{}
	filters.Set("Header2:1/2")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		HeaderHashFilters: filters,
	})

	payload := func(header []byte) []byte {
		return []byte("POST / HTTP/1.1\r\n" + string(header) + "Content-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	if p := modifier.Rewrite(payload([]byte(""))); len(p) == 0 {
		t.Error("Request should pass filters if Header does not exist")
	}

	if p := modifier.Rewrite(payload([]byte("Header2: 3\r\n"))); len(p) > 0 {
		t.Error("Request should not pass filters, Header2 hash too high")
	}

	if p := modifier.Rewrite(payload([]byte("Header2: 1\r\n"))); len(p) == 0 {
		t.Error("Request should pass filters")
	}
}

func TestHTTPModifierParamHashFilters(t *testing.T) {
	filters := HTTPHashFilters{}
	filters.Set("user_id:1/2")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		ParamHashFilters: filters,
	})

	payload := func(value []byte) []byte {
		return []byte("POST /" + string(value) + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	if p := modifier.Rewrite(payload([]byte(""))); len(p) == 0 {
		t.Error("Request should pass filters if param does not exist")
	}

	if p := modifier.Rewrite(payload([]byte("?user_id=3"))); len(p) > 0 {
		t.Error("Request should not pass filters", string(p))
	}

	if p := modifier.Rewrite(payload([]byte("?user_id=1"))); len(p) == 0 {
		t.Error("Request should pass filters")
	}
}

func TestHTTPModifierHeaders(t *testing.T) {
	headers := HTTPHeaders{}
	headers.Set("Header1:1")
	headers.Set("Host:localhost")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		Headers: headers,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	newPayload := []byte("POST /post HTTP/1.1\r\nHeader1: 1\r\nContent-Length: 7\r\nHost: localhost\r\n\r\na=1&b=2")

	if payload = modifier.Rewrite(payload); !bytes.Equal(payload, newPayload) {
		t.Error("Should update request headers", string(payload))
	}
}

func TestHTTPModifierURLRegexp(t *testing.T) {
	filters := HTTPURLRegexp{}
	filters.Set("ad_ids=1677")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		URLRegexp: filters,
	})

	payload := func(url string) []byte {
		return []byte("POST " + url + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	//if len(modifier.Rewrite(payload("/getAd?ad_type=7&spread_type=1&template=0&dBrand=HONOR&ec=utf-8&device_id=f6518c93a5c9b00a&dlid=7%2C527%2C535%2C35076&urlcid3=&sku_id=100200300265&xAPINeoType=-1&networkType=wifi&eid=eidA20178122fcscCcvkrXJ4QQOQ14xwZSpgUwnL5jk7NPIOFEeycHvadjdQ0TGrji2iBkdqOnr8GVyrzKFfJbBQfmUDO%2FO3C61M95hW8PwkkpYRsdBl&pop=&version_number=0&seedSource=&clientUuid=f6518c93a5c9b00a&aid=f6518c93a5c9b00a&area=7%2C527%2C535%2C35076&mixer_uuid=f6518c93a5c9b00a&columnFlag=&presentSkus=&imup=&clientPageId=&build=101729&local_date_time=2025-10-29+08%3A27%3A33&urlcid2=&similarTopSku=&seedCat=9987%3B653%3B655&rlid=7%2C527%2C535%2C35076&dModel=PGT-AN00&upg=000&bssid=01a54c992a3a676e47601bb949325e39&securityToken=JD022145b9HiXdJ3grqZ1761697612074072luTNQZ4ymNp_JHqT8Lv7aOwkmYZ0DiYiIDPYqdVLSkLcuLYaeJNrvRIGPivG7Ty89pTNnOIHszJwxnxAftNNA0tyt9la%7EBApXW5N1ULv9C9q4Q8kfutLLJaXe7MOBWgueItyVX9xJ1PdZfQrz1sRbbrwzyK5F4fY0_e6Tn7WO4fw&rcd=114.399385%2C34.044062&device_type=16384&clientMode=0&version=15.2.70&urlcid1=&test_category=0&ulid=7%2C527%2C535%2C35076&skus=&language=zh_CN&mobile_type=2&specialRecommendType=&lim=24&bkt=2&last_click_id=&sameTopSku=&fullcutflag=1&client=android&p=902008&noSkuShowType=&clientChannel=2&request_id=10192110821-141010-1761697654826&region=CN&ad_ids=1677%3A24&source_p=&currency=CNY&recommend_ext=%7B%22adver%22%3A%220%22%2C%22event_id%22%3A%22Searchlist_Productid%22%2C%22on_site%22%3A%221%22%2C%22page_id%22%3A%22Search_ProductList%22%2C%22query%22%3A%22%E8%8D%A3%E8%80%8070plus%E7%95%85%E7%8E%A9%22%2C%22search_cat%22%3A%229987%2C653%2C655%22%7D&page=0&app_info=2688*1224%5EPGT-AN00%5Eandroid%5E15%5E15.2.70%5Ewifi&isReviewVersion=&dcd=114.39919%2C34.043599&prstate=0&idfa=&osVersion=15&time_zone_offset=%2B08%3A00&shopid=1000441041&pin=jd_ZCClPCGhxwkn&umg=50&ucd=&detailPageSource=search&uemps=0-2-0&fCnt=0&iplocation=171.15.183.208&location_info=7-527-535-35076&fullcutnomarkflag=1&timeZone=&matExtMsg=&bybtTraffic=1&flt=0&palantir_expids=Z%5ERA%7CMIXTAG_Z%5ERAR%2CZ%5ERA_NN_YXRAZ_R%2CZ%5ERA_NN_bybtRecAdMixer_R%2CseckillNight_78761_preB%2CZ%5ERA_NN_RECAD01-3211_L73340%2Crecomfront_84685_exchange_loc_base%2CZ%5ERA_NN_RARecServerRouteLaye-3604_L78604%2CMIXTAG_RAR%2CRA_RA_YXM_R%2CRA_RA_BaseLayer902008_R%7C&mixer_flow=2&mixer=1&from=3&ad_ratio=40&organic_context=100195263757%2C100216577096%2C100236385816%2C100195263753%2C100168156911%2C100216577102%2C100236871946%2C100164178527%2C100212982838%2C100195263751%2C100198718422%2C100210604060%2C100195263771%2C100216577104%2C100210235376%2C100200300265%2C100275758824%2C100236385792%2C100275758806%2C100216577122%2C100198718578%2C100165763299%2C100195263779%2C100236385834%2C100236385832%2C100156258275%2C100181700713%2C100168156893%2C100275758790%2C100236385826%2C100275758792%2C100264777490%2C100236385824%2C100198823676%2C100257209568%2C100236385830%2C100149433091%2C100149433115%2C100181374477%2C100156258445%2C100163932533%2C100216577046%2C100257209602%2C100167328313%2C100215333434%2C100264777546%2C100257209534%2C100198718394%2C100264777548%2C100181374517%2C100236871910%2C100216577058%2C100264777540%2C100210235398%2C100264777562%2C100181374499%2C100264777564%2C100181374501%2C100181374505%2C100212982850&gateway_cmd=5&forcebot=1"))) == 0 {
	//	t.Error("Should pass url")
	//}
	for i := 0; i < 100000; i++ {
		if len(modifier.Rewrite(payload("/getAd?ad_type=7&spread_type=1&template=0&dBrand=vivo&ec=utf-8&device_id=43489b28e57118f6&dlid=29%2C2580%2C2583%2C21934&urlcid3=&sku_id=10132063844699&xAPINeoType=-1&networkType=wifi&eid=eidA2749812308s4%2FNnEPZdLTpWwJjy%2FuEbakPzZxdD6kkytk65uGIEVW7iR7tBc8VcwandhRNkPtUiNGhXQqSOxjcUUjD233LcttM02mdCJQZOXiQ5Q&pop=&version_number=1&seedSource=&clientUuid=43489b28e57118f6&aid=43489b28e57118f6&area=29%2C2580%2C2583%2C21934&mixer_uuid=43489b28e57118f6&columnFlag=&presentSkus=&imup=&clientPageId=&build=101729&local_date_time=2025-10-29+08%3A42%3A41&urlcid2=&similarTopSku=&seedCat=34767%3B38379%3B38380&rlid=0%2C0%2C0%2C0&dModel=V2359A&upg=000&bssid=unknown&securityToken=JD022145b9m4uLrCot5E176169846272307q8GWIm9IU6C1rQ8eqvKfr6B90x6X_U8a_t2bjQmM3-JUKKlF7FrIzf442R5URFEktflthamCRiz6UZEpELDJ-g129xl0v%7EBApXWltVhLv9C9q4Q8kfutLLJaXe7hflBOacwH9lK9xJ1PdZfQpzbmx_soSXjN69YWpxUkqbnk0o2EA&rcd=0.0%2C0.0&device_type=16384&clientMode=0&version=15.2.70&urlcid1=&test_category=1&ulid=29%2C2580%2C2583%2C21934&skus=&language=zh_CN&mobile_type=2&specialRecommendType=&lim=24&bkt=10&last_click_id=&sameTopSku=&fullcutflag=1&client=android&p=902008&noSkuShowType=&clientChannel=2&request_id=10192108874-158114-1761698562337&region=CN&ad_ids=1677%3A24&source_p=&currency=CNY&recommend_ext=&page=0&app_info=2400*1080%5EV2359A%5Eandroid%5E15%5E15.2.70%5Ewifi&isReviewVersion=&dcd=101.678452%2C36.951725&prstate=0&idfa=&osVersion=15&time_zone_offset=%2B08%3A00&shopid=13031260&pin=jd_zmiegMnydIKI&umg=50&ucd=&detailPageSource=m_destination_page&uemps=0-2-0&fCnt=0&iplocation=118.213.99.79&location_info=29-2580-2583-21934&fullcutnomarkflag=1&timeZone=&matExtMsg=&bybtTraffic=1&flt=0&palantir_expids=Z%5ERA%7CMIXTAG_Z%5ERAR%2CZ%5ERA_NN_YXRAZ_R%2CZ%5ERA_NN_bybtRecAdMixer_R%2Crecomfront_84685_exp8%2CZ%5ERA_NN_RARecServerRouteLaye-3571_L78109%2CseckillNight_78761_preA%2CZ%5ERA_NN_RECAD01-3208_L73325%2CMIXTAG_RAR%2CRA_RA_YXM_R%2CRA_RA_BaseLayer902008_R%7C&mixer_flow=2&mixer=1&from=3&ad_ratio=40&organic_context=10118706992348%2C10148022256511%2C10156923061370%2C10150758585963%2C10115338554123%2C10137913055774%2C10142433750275%2C10184680792465%2C10125368295127%2C10117576619940%2C10125368295126%2C10125368295125%2C10125368295123%2C10116523221096%2C10123172891829%2C10125368295121%2C10117576619939%2C10125368295120%2C10158324750580%2C10126352285744%2C10122036405744%2C10155783034305%2C10123862852384%2C10140907038010%2C10119916018279%2C10133290631339%2C10119916018277%2C10150968417864%2C10160300596745%2C10123861143971%2C10140153732430%2C10121189871854%2C10117437388362%2C10158982883823%2C10115067968253%2C10162506108003%2C10136712765277%2C10112022541181%2C10114277019393%2C10116523281318%2C10117891615029%2C10168654387172%2C10133200691634%2C10134479334759%2C10122849998601%2C10166863565608%2C10141987930608%2C10168693380100%2C10145418929741%2C10088924020082%2C10154102093949%2C10123807263284%2C10123164060610%2C10118284079097%2C10159915135085%2C10123693985762%2C10170609880492%2C10140507270295%2C10151606468770%2C10172170388772&gateway_cmd=5&forcebot=1"))) == 0 {
			t.Error("Should pass url")
		}
	}

	//if len(modifier.Rewrite(payload("/v1/api/test"))) == 0 {
	//	t.Error("Should pass url")
	//}
	//
	//if len(modifier.Rewrite(payload("/other"))) > 0 {
	//	t.Error("Should not pass url")
	//}
}

func TestHTTPModifierURLParamFilter(t *testing.T) {
	filters := HTTPParamFilters{}
	filters.Set("ad_ids=")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		ParamFilters: filters,
	})

	payload := func(url string) []byte {
		return []byte("POST " + url + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	//if len(modifier.Rewrite(payload("/getAd?ad_type=7&spread_type=1&template=0&dBrand=HONOR&ec=utf-8&device_id=f6518c93a5c9b00a&dlid=7%2C527%2C535%2C35076&urlcid3=&sku_id=100200300265&xAPINeoType=-1&networkType=wifi&eid=eidA20178122fcscCcvkrXJ4QQOQ14xwZSpgUwnL5jk7NPIOFEeycHvadjdQ0TGrji2iBkdqOnr8GVyrzKFfJbBQfmUDO%2FO3C61M95hW8PwkkpYRsdBl&pop=&version_number=0&seedSource=&clientUuid=f6518c93a5c9b00a&aid=f6518c93a5c9b00a&area=7%2C527%2C535%2C35076&mixer_uuid=f6518c93a5c9b00a&columnFlag=&presentSkus=&imup=&clientPageId=&build=101729&local_date_time=2025-10-29+08%3A27%3A33&urlcid2=&similarTopSku=&seedCat=9987%3B653%3B655&rlid=7%2C527%2C535%2C35076&dModel=PGT-AN00&upg=000&bssid=01a54c992a3a676e47601bb949325e39&securityToken=JD022145b9HiXdJ3grqZ1761697612074072luTNQZ4ymNp_JHqT8Lv7aOwkmYZ0DiYiIDPYqdVLSkLcuLYaeJNrvRIGPivG7Ty89pTNnOIHszJwxnxAftNNA0tyt9la%7EBApXW5N1ULv9C9q4Q8kfutLLJaXe7MOBWgueItyVX9xJ1PdZfQrz1sRbbrwzyK5F4fY0_e6Tn7WO4fw&rcd=114.399385%2C34.044062&device_type=16384&clientMode=0&version=15.2.70&urlcid1=&test_category=0&ulid=7%2C527%2C535%2C35076&skus=&language=zh_CN&mobile_type=2&specialRecommendType=&lim=24&bkt=2&last_click_id=&sameTopSku=&fullcutflag=1&client=android&p=902008&noSkuShowType=&clientChannel=2&request_id=10192110821-141010-1761697654826&region=CN&ad_ids=1677%3A24&source_p=&currency=CNY&recommend_ext=%7B%22adver%22%3A%220%22%2C%22event_id%22%3A%22Searchlist_Productid%22%2C%22on_site%22%3A%221%22%2C%22page_id%22%3A%22Search_ProductList%22%2C%22query%22%3A%22%E8%8D%A3%E8%80%8070plus%E7%95%85%E7%8E%A9%22%2C%22search_cat%22%3A%229987%2C653%2C655%22%7D&page=0&app_info=2688*1224%5EPGT-AN00%5Eandroid%5E15%5E15.2.70%5Ewifi&isReviewVersion=&dcd=114.39919%2C34.043599&prstate=0&idfa=&osVersion=15&time_zone_offset=%2B08%3A00&shopid=1000441041&pin=jd_ZCClPCGhxwkn&umg=50&ucd=&detailPageSource=search&uemps=0-2-0&fCnt=0&iplocation=171.15.183.208&location_info=7-527-535-35076&fullcutnomarkflag=1&timeZone=&matExtMsg=&bybtTraffic=1&flt=0&palantir_expids=Z%5ERA%7CMIXTAG_Z%5ERAR%2CZ%5ERA_NN_YXRAZ_R%2CZ%5ERA_NN_bybtRecAdMixer_R%2CseckillNight_78761_preB%2CZ%5ERA_NN_RECAD01-3211_L73340%2Crecomfront_84685_exchange_loc_base%2CZ%5ERA_NN_RARecServerRouteLaye-3604_L78604%2CMIXTAG_RAR%2CRA_RA_YXM_R%2CRA_RA_BaseLayer902008_R%7C&mixer_flow=2&mixer=1&from=3&ad_ratio=40&organic_context=100195263757%2C100216577096%2C100236385816%2C100195263753%2C100168156911%2C100216577102%2C100236871946%2C100164178527%2C100212982838%2C100195263751%2C100198718422%2C100210604060%2C100195263771%2C100216577104%2C100210235376%2C100200300265%2C100275758824%2C100236385792%2C100275758806%2C100216577122%2C100198718578%2C100165763299%2C100195263779%2C100236385834%2C100236385832%2C100156258275%2C100181700713%2C100168156893%2C100275758790%2C100236385826%2C100275758792%2C100264777490%2C100236385824%2C100198823676%2C100257209568%2C100236385830%2C100149433091%2C100149433115%2C100181374477%2C100156258445%2C100163932533%2C100216577046%2C100257209602%2C100167328313%2C100215333434%2C100264777546%2C100257209534%2C100198718394%2C100264777548%2C100181374517%2C100236871910%2C100216577058%2C100264777540%2C100210235398%2C100264777562%2C100181374499%2C100264777564%2C100181374501%2C100181374505%2C100212982850&gateway_cmd=5&forcebot=1"))) == 0 {
	//	t.Error("Should pass url")
	//}
	for i := 0; i < 100000; i++ {

		if len(modifier.Rewrite(payload("/getAd?ad_type=7&spread_type=1&template=0&dBrand=vivo&ec=utf-8&device_id=43489b28e57118f6&dlid=29%2C2580%2C2583%2C21934&urlcid3=&sku_id=10132063844699&xAPINeoType=-1&networkType=wifi&eid=eidA2749812308s4%2FNnEPZdLTpWwJjy%2FuEbakPzZxdD6kkytk65uGIEVW7iR7tBc8VcwandhRNkPtUiNGhXQqSOxjcUUjD233LcttM02mdCJQZOXiQ5Q&pop=&version_number=1&seedSource=&clientUuid=43489b28e57118f6&aid=43489b28e57118f6&area=29%2C2580%2C2583%2C21934&mixer_uuid=43489b28e57118f6&columnFlag=&presentSkus=&imup=&clientPageId=&build=101729&local_date_time=2025-10-29+08%3A42%3A41&urlcid2=&similarTopSku=&seedCat=34767%3B38379%3B38380&rlid=0%2C0%2C0%2C0&dModel=V2359A&upg=000&bssid=unknown&securityToken=JD022145b9m4uLrCot5E176169846272307q8GWIm9IU6C1rQ8eqvKfr6B90x6X_U8a_t2bjQmM3-JUKKlF7FrIzf442R5URFEktflthamCRiz6UZEpELDJ-g129xl0v%7EBApXWltVhLv9C9q4Q8kfutLLJaXe7hflBOacwH9lK9xJ1PdZfQpzbmx_soSXjN69YWpxUkqbnk0o2EA&rcd=0.0%2C0.0&device_type=16384&clientMode=0&version=15.2.70&urlcid1=&test_category=1&ulid=29%2C2580%2C2583%2C21934&skus=&language=zh_CN&mobile_type=2&specialRecommendType=&lim=24&bkt=10&last_click_id=&sameTopSku=&fullcutflag=1&client=android&p=902008&noSkuShowType=&clientChannel=2&request_id=10192108874-158114-1761698562337&region=CN&ad_ids=1677%3A24&source_p=&currency=CNY&recommend_ext=&page=0&app_info=2400*1080%5EV2359A%5Eandroid%5E15%5E15.2.70%5Ewifi&isReviewVersion=&dcd=101.678452%2C36.951725&prstate=0&idfa=&osVersion=15&time_zone_offset=%2B08%3A00&shopid=13031260&pin=jd_zmiegMnydIKI&umg=50&ucd=&detailPageSource=m_destination_page&uemps=0-2-0&fCnt=0&iplocation=118.213.99.79&location_info=29-2580-2583-21934&fullcutnomarkflag=1&timeZone=&matExtMsg=&bybtTraffic=1&flt=0&palantir_expids=Z%5ERA%7CMIXTAG_Z%5ERAR%2CZ%5ERA_NN_YXRAZ_R%2CZ%5ERA_NN_bybtRecAdMixer_R%2Crecomfront_84685_exp8%2CZ%5ERA_NN_RARecServerRouteLaye-3571_L78109%2CseckillNight_78761_preA%2CZ%5ERA_NN_RECAD01-3208_L73325%2CMIXTAG_RAR%2CRA_RA_YXM_R%2CRA_RA_BaseLayer902008_R%7C&mixer_flow=2&mixer=1&from=3&ad_ratio=40&organic_context=10118706992348%2C10148022256511%2C10156923061370%2C10150758585963%2C10115338554123%2C10137913055774%2C10142433750275%2C10184680792465%2C10125368295127%2C10117576619940%2C10125368295126%2C10125368295125%2C10125368295123%2C10116523221096%2C10123172891829%2C10125368295121%2C10117576619939%2C10125368295120%2C10158324750580%2C10126352285744%2C10122036405744%2C10155783034305%2C10123862852384%2C10140907038010%2C10119916018279%2C10133290631339%2C10119916018277%2C10150968417864%2C10160300596745%2C10123861143971%2C10140153732430%2C10121189871854%2C10117437388362%2C10158982883823%2C10115067968253%2C10162506108003%2C10136712765277%2C10112022541181%2C10114277019393%2C10116523281318%2C10117891615029%2C10168654387172%2C10133200691634%2C10134479334759%2C10122849998601%2C10166863565608%2C10141987930608%2C10168693380100%2C10145418929741%2C10088924020082%2C10154102093949%2C10123807263284%2C10123164060610%2C10118284079097%2C10159915135085%2C10123693985762%2C10170609880492%2C10140507270295%2C10151606468770%2C10172170388772&gateway_cmd=5&forcebot=1"))) == 0 {
			t.Error("Should pass url")
		}
	}

	//if len(modifier.Rewrite(payload("/v1/api/test"))) == 0 {
	//	t.Error("Should pass url")
	//}
	//
	//if len(modifier.Rewrite(payload("/other"))) > 0 {
	//	t.Error("Should not pass url")
	//}
}

func TestHTTPModifierURLNegativeRegexp(t *testing.T) {
	filters := HTTPURLRegexp{}
	filters.Set("/restricted1")
	filters.Set("/some/restricted2")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		URLNegativeRegexp: filters,
	})

	payload := func(url string) []byte {
		return []byte("POST " + url + " HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	}

	if len(modifier.Rewrite(payload("/v1/app/test"))) == 0 {
		t.Error("Should pass url")
	}

	if len(modifier.Rewrite(payload("/restricted1"))) > 0 {
		t.Error("Should not pass url")
	}

	if len(modifier.Rewrite(payload("/some/restricted2"))) > 0 {
		t.Error("Should not pass url")
	}
}

func TestHTTPModifierSetHeader(t *testing.T) {
	filters := HTTPHeaders{}
	filters.Set("User-Agent:Gor")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		Headers: filters,
	})

	payload := []byte("POST /post HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter := []byte("POST /post HTTP/1.1\r\nUser-Agent: Gor\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = modifier.Rewrite(payload); !bytes.Equal(payloadAfter, payload) {
		t.Error("Should add new header", string(payload))
	}
}

func TestHTTPModifierSetParam(t *testing.T) {
	filters := HTTPParams{}
	filters.Set("api_key=1")

	modifier := NewHTTPModifier(&HTTPModifierConfig{
		Params: filters,
	})

	payload := []byte("POST /post?api_key=1234 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")
	payloadAfter := []byte("POST /post?api_key=1 HTTP/1.1\r\nContent-Length: 7\r\nHost: www.w3.org\r\n\r\na=1&b=2")

	if payload = modifier.Rewrite(payload); !bytes.Equal(payloadAfter, payload) {
		t.Error("Should override param", string(payload))
	}
}
