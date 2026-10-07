package atmos

import "math"

// The orbit in the past. The tilt of the spin, the eccentricity of the orbit
// and where on it the planet is nearest the sun all wander, under the pull of
// the other planets and of the moon on the equatorial bulge, with periods of
// some forty-one thousand years for the tilt, a hundred thousand and four
// hundred thousand for the eccentricity, and nineteen to twenty-three
// thousand for the precession of perihelion against the equinoxes. They are
// what Milankovitch's ice ages are paced by.
//
// Berger (1978) put them as trigonometric series in time: the obliquity a
// constant and 47 cosines, the eccentricity and the longitude of perihelion
// against the fixed equinox from 19 terms in e sin and e cos, and the
// precession of the equinoxes a rate, a constant and 78 sines. They are good
// to a million years either side of 1950. Berger and Loutre (1991) is the
// newer solution and good to five million, but it is a numerical one,
// tabulated every thousand years rather than a series, and its table of five
// thousand rows is not carried here; over the last million years the two
// agree to well within what the energy balance can tell apart (Berger and
// Loutre, 1991, compare them).
//
// The terms below are Berger's (1978) full series as the Community Earth
// System Model's shr_orb_mod.F90 transcribes them (from Berger, 1978, A
// Simple Algorithm to Compute Long-Term Variations of Daily Insolation,
// Contribution 18, Institut d'Astronomie et de Géophysique, Louvain-la-Neuve,
// and Berger, 1978, J. Atmos. Sci. 35, 2362-2367), every one of them.

// bergerTerm is one term of a series: its amplitude (arc seconds for the
// obliquity and the precession, a number for the eccentricity), its rate in
// arc seconds a year, and its phase in degrees.
type bergerTerm struct{ amp, rate, phase float64 }

// The series' constants (Berger, 1978): the obliquity's mean, in degrees, and
// the general precession's rate, in arc seconds a year, and constant, in
// degrees.
const (
	bergerObliquity       = 23.320556
	bergerPrecessionRate  = 50.439273
	bergerPrecessionConst = 3.392506
	arcSecond             = 1.0 / 3600 // of a degree
	degree                = math.Pi / 180
)

// OrbitBefore is f with the orbit the planet had years before 1950: its
// eccentricity, obliquity and longitude of perihelion from Berger's (1978)
// series, and f's sun and air as they were. The series is good to a million
// years either side; further out it still gives an orbit, but not the one
// there was.
//
// OrbitBefore(0) is the real orbit of 1950 - an eccentricity of 0.01672, a
// tilt of 23.446 degrees, a perihelion at 102.04 - and not Today's, whose
// figures are FAO-56's rounding of it: see todayEccentricity.
func (f Forcing) OrbitBefore(years float64) Forcing {
	f = f.OrDefault()
	t := -years // the series run forward in time from 1950
	var ob float64
	for _, c := range obliquityTerms {
		ob += c.amp * arcSecond * math.Cos((c.rate*arcSecond*t+c.phase)*degree)
	}
	var ec, es float64
	for _, c := range eccentricityTerms {
		a := (c.rate*arcSecond*t + c.phase) * degree
		ec += c.amp * math.Cos(a)
		es += c.amp * math.Sin(a)
	}
	// The longitude of perihelion against the fixed equinox, and the general
	// precession that carries the equinox round against it.
	fixed := math.Atan2(es, ec) / degree
	var mv float64
	for _, c := range precessionTerms {
		mv += c.amp * arcSecond * math.Sin((c.rate*arcSecond*t+c.phase)*degree)
	}
	peri := math.Mod(fixed+bergerPrecessionRate*arcSecond*t+bergerPrecessionConst+mv, 360)
	if peri < 0 {
		peri += 360
	}
	f.Eccentricity = math.Hypot(ec, es)
	f.Obliquity = (bergerObliquity + ob) * degree
	f.Perihelion = peri * degree
	return f
}

// obliquityTerms are the cosines of the obliquity.
var obliquityTerms = [...]bergerTerm{
	{-2462.2214466, 31.609974, 251.9025},
	{-857.3232075, 32.620504, 280.8325},
	{-629.3231835, 24.172203, 128.3057},
	{-414.2804924, 31.983787, 292.7252},
	{-311.7632587, 44.828336, 15.3747},
	{308.9408604, 30.973257, 263.7951},
	{-162.5533601, 43.668246, 308.4258},
	{-116.1077911, 32.246691, 240.0099},
	{101.1189923, 30.599444, 222.9725},
	{-67.6856209, 42.681324, 268.7809},
	{24.9079067, 43.836462, 316.7998},
	{22.5811241, 47.439436, 319.6024},
	{-21.1648355, 63.219948, 143.8050},
	{-15.6549876, 64.230478, 172.7351},
	{15.3936813, 1.010530, 28.9300},
	{14.6660938, 7.437771, 123.5968},
	{-11.7273029, 55.782177, 20.2082},
	{10.2742696, 0.373813, 40.8226},
	{6.4914588, 13.218362, 123.4722},
	{5.8539148, 62.583231, 155.6977},
	{-5.4872205, 63.593761, 184.6277},
	{-5.4290191, 76.438310, 267.2772},
	{5.1609570, 45.815258, 55.0196},
	{5.0786314, 8.448301, 152.5268},
	{-4.0735782, 56.792707, 49.1382},
	{3.7227167, 49.747842, 204.6609},
	{3.3971932, 12.058272, 56.5233},
	{-2.8347004, 75.278220, 200.3284},
	{-2.6550721, 65.241008, 201.6651},
	{-2.5717867, 64.604291, 213.5577},
	{-2.4712188, 1.647247, 17.0374},
	{2.4625410, 7.811584, 164.4194},
	{2.2464112, 12.207832, 94.5422},
	{-2.0755511, 63.856665, 131.9124},
	{-1.9713669, 56.155990, 61.0309},
	{-1.8813061, 77.448840, 296.2073},
	{-1.8468785, 6.801054, 135.4894},
	{1.8186742, 62.209418, 114.8750},
	{1.7601888, 20.656133, 247.0691},
	{-1.5428851, 48.344406, 256.6114},
	{1.4738838, 55.145460, 32.1008},
	{-1.4593669, 69.000539, 143.6804},
	{1.4192259, 11.071350, 16.8784},
	{-1.1818980, 74.291298, 160.6835},
	{1.1756474, 11.047742, 27.5932},
	{-1.1316126, 0.636717, 348.1074},
	{1.0896928, 12.844549, 82.6496},
}

// eccentricityTerms are the terms of e cos and e sin of the longitude of
// perihelion against the fixed equinox (Berger, 1978).
var eccentricityTerms = [...]bergerTerm{
	{0.01860798, 4.2072050, 28.620089},
	{0.01627522, 7.3460910, 193.788772},
	{-0.01300660, 17.8572630, 308.307024},
	{0.00988829, 17.2205460, 320.199637},
	{-0.00336700, 16.8467330, 279.376984},
	{0.00333077, 5.1990790, 87.195000},
	{-0.00235400, 18.2310760, 349.129677},
	{0.00140015, 26.2167580, 128.443387},
	{0.00100700, 6.3591690, 154.143880},
	{0.00085700, 16.2100160, 291.269597},
	{0.00064990, 3.0651810, 114.860583},
	{0.00059900, 16.5838290, 332.092251},
	{0.00037800, 18.4939800, 296.414411},
	{-0.00033700, 6.1909530, 145.769910},
	{0.00027600, 18.8677930, 337.237063},
	{0.00018200, 17.4255670, 152.092288},
	{-0.00017400, 6.1860010, 126.839891},
	{-0.00012400, 18.4174410, 210.667199},
	{0.00001250, 0.6678630, 72.108838},
}

// precessionTerms are the sines of the general precession.
var precessionTerms = [...]bergerTerm{
	{7391.0225890, 31.609974, 251.9025},
	{2555.1526947, 32.620504, 280.8325},
	{2022.7629188, 24.172203, 128.3057},
	{-1973.6517951, 0.636717, 348.1074},
	{1240.2321818, 31.983787, 292.7252},
	{953.8679112, 3.138886, 165.1686},
	{-931.7537108, 30.973257, 263.7951},
	{872.3795383, 44.828336, 15.3747},
	{606.3544732, 0.991874, 58.5749},
	{-496.0274038, 0.373813, 40.8226},
	{456.9608039, 43.668246, 308.4258},
	{346.9462320, 32.246691, 240.0099},
	{-305.8412902, 30.599444, 222.9725},
	{249.6173246, 2.147012, 106.5937},
	{-199.1027200, 10.511172, 114.5182},
	{191.0560889, 42.681324, 268.7809},
	{-175.2936572, 13.650058, 279.6869},
	{165.9068833, 0.986922, 39.6448},
	{161.1285917, 9.874455, 126.4108},
	{139.7878093, 13.013341, 291.5795},
	{-133.5228399, 0.262904, 307.2848},
	{117.0673811, 0.004952, 18.9300},
	{104.6907281, 1.142024, 273.7596},
	{95.3227476, 63.219948, 143.8050},
	{86.7824524, 0.205021, 191.8927},
	{86.0857729, 2.151964, 125.5237},
	{70.5893698, 64.230478, 172.7351},
	{-69.9719343, 43.836462, 316.7998},
	{-62.5817473, 47.439436, 319.6024},
	{61.5450059, 1.384343, 69.7526},
	{-57.9364011, 7.437771, 123.5968},
	{57.1899832, 18.829299, 217.6432},
	{-57.0236109, 9.500642, 85.5882},
	{-54.2119253, 0.431696, 156.2147},
	{53.2834147, 1.160090, 66.9489},
	{52.1223575, 55.782177, 20.2082},
	{-49.0059908, 12.639528, 250.7568},
	{-48.3118757, 1.155138, 48.0188},
	{-45.4191685, 0.168216, 8.3739},
	{-42.2357920, 1.647247, 17.0374},
	{-34.7971099, 10.884985, 155.3409},
	{34.4623613, 5.610937, 94.1709},
	{-33.8356643, 12.658184, 221.1120},
	{33.6689362, 1.010530, 28.9300},
	{-31.2521586, 1.983748, 117.1498},
	{-30.8798701, 14.023871, 320.5095},
	{28.4640769, 0.560178, 262.3602},
	{-27.1960802, 1.273434, 336.2148},
	{27.0860736, 12.021467, 233.0046},
	{-26.3437456, 62.583231, 155.6977},
	{24.7253740, 63.593761, 184.6277},
	{24.6732126, 76.438310, 267.2772},
	{24.4272733, 4.280910, 78.9281},
	{24.0127327, 13.218362, 123.4722},
	{21.7150294, 17.818769, 188.7132},
	{-21.5375347, 8.359495, 180.1364},
	{18.1148363, 56.792707, 49.1382},
	{-16.9603104, 8.448301, 152.5268},
	{-16.1765215, 1.978796, 98.2198},
	{15.5567653, 8.863925, 97.4808},
	{15.4846529, 0.186365, 221.5376},
	{15.2150632, 8.996212, 168.2438},
	{14.5047426, 6.771027, 161.1199},
	{-14.3873316, 45.815258, 55.0196},
	{13.1351419, 12.002811, 262.6495},
	{12.8776311, 75.278220, 200.3284},
	{11.9867234, 65.241008, 201.6651},
	{11.9385578, 18.870667, 294.6547},
	{11.7030822, 22.009553, 99.8233},
	{11.6018181, 64.604291, 213.5577},
	{-11.2617293, 11.498094, 154.1631},
	{-10.4664199, 0.578834, 232.7153},
	{10.4333970, 9.237738, 138.3034},
	{-10.2377466, 49.747842, 204.6609},
	{10.1934446, 2.147012, 106.5938},
	{-10.1280191, 1.196895, 250.4676},
	{10.0289441, 2.133898, 332.3345},
	{-10.0034259, 0.173168, 27.3039},
}
