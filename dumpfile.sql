-- MySQL dump 10.13  Distrib 5.5.24, for debian-linux-gnu (x86_64)

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `avatars`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `avatars` (
  `name` varchar(50) NOT NULL,
  `id` int(1) unsigned NOT NULL AUTO_INCREMENT,
  `owner` varchar(255) NOT NULL,
  `HeadType` smallint(5) unsigned NOT NULL DEFAULT '0',
  `BodyType` smallint(5) unsigned NOT NULL DEFAULT '0',
  `PositionX` double NOT NULL DEFAULT '0',
  `PositionY` double NOT NULL DEFAULT '0',
  `PositionZ` double NOT NULL DEFAULT '2',
  `isFlying` tinyint(1) NOT NULL DEFAULT '0',
  `isClimbing` tinyint(1) NOT NULL DEFAULT '0',
  `isDead` tinyint(1) NOT NULL DEFAULT '0',
  `DirHor` float NOT NULL DEFAULT '0',
  `DirVert` float NOT NULL DEFAULT '0',
  `AdminLevel` int(1) unsigned NOT NULL DEFAULT '0',
  `Level` int(1) unsigned NOT NULL DEFAULT '0',
  `Experience` double NOT NULL DEFAULT '0',
  `HitPoints` double NOT NULL DEFAULT '1',
  `Mana` double NOT NULL DEFAULT '0',
  `Kills` int(1) NOT NULL DEFAULT '0',
  `BlocksAdded` int(10) unsigned NOT NULL DEFAULT '0',
  `BlocksRemoved` int(10) unsigned NOT NULL DEFAULT '0',
  `HomeX` double NOT NULL DEFAULT '0',
  `HomeY` double NOT NULL DEFAULT '0',
  `HomeZ` double NOT NULL DEFAULT '0',
  `TargetX` double NOT NULL DEFAULT '0',
  `TargetY` double NOT NULL DEFAULT '0',
  `TargetZ` double NOT NULL DEFAULT '0',
  `ReviveX` double NOT NULL DEFAULT '0',
  `ReviveY` double NOT NULL DEFAULT '0',
  `ReviveZ` double NOT NULL DEFAULT '0',
  `maxchunks` int(1) NOT NULL DEFAULT '-1',
  `TimeOnline` int(10) unsigned NOT NULL DEFAULT '0',
  `jsonstring` varchar(1000) NOT NULL,
  `lastseen` date NOT NULL DEFAULT '0000-00-00',
  `Inventory` blob NOT NULL,
  `TScoreTotal` double NOT NULL DEFAULT '0',
  `TScoreBalance` double NOT NULL DEFAULT '0',
  `TScoreTime` int(11) NOT NULL DEFAULT '0',
  UNIQUE KEY `name` (`name`),
  UNIQUE KEY `id` (`id`)
) ENGINE=MyISAM AUTO_INCREMENT=55 DEFAULT CHARSET=latin1;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `chunkdata`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `chunkdata` (
  `x` int(1) NOT NULL,
  `y` int(1) NOT NULL,
  `z` int(1) NOT NULL,
  `avatarID` int(1) unsigned NOT NULL DEFAULT '0',
  `json` varchar(100) NOT NULL DEFAULT '',
  `reqrelease` tinyint(1) NOT NULL DEFAULT '0',
  `support` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`x`,`y`,`z`),
  KEY `avatarID` (`avatarID`)
) ENGINE=MyISAM DEFAULT CHARSET=latin1;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `friends`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `friends` (
  `avatar` int(1) NOT NULL,
  `friend` int(1) NOT NULL,
  KEY `avatar` (`avatar`)
) ENGINE=MyISAM DEFAULT CHARSET=latin1;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `users`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `users` (
  `email` varchar(255) NOT NULL,
  `password` char(32) NOT NULL DEFAULT '00000000000000000000000000000000',
  `lastseen` date NOT NULL,
  `lastaddr` varchar(20) NOT NULL DEFAULT '0.0.0.0',
  `licensekey` char(20) NOT NULL,
  UNIQUE KEY `email` (`email`)
) ENGINE=MyISAM DEFAULT CHARSET=latin1;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2012-09-27  7:43:19
